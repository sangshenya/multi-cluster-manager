package resource_binding_controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"time"

	"k8s.io/client-go/rest"

	"k8s.io/apimachinery/pkg/labels"

	"k8s.io/apimachinery/pkg/runtime/schema"

	"harmonycloud.cn/multi-cluster-manager/pkg/apis/multicluster/common"

	"k8s.io/apimachinery/pkg/runtime"

	"harmonycloud.cn/multi-cluster-manager/pkg/apis/multicluster/v1alpha1"
	clientset "harmonycloud.cn/multi-cluster-manager/pkg/client/clientset/versioned"
	managerScheme "harmonycloud.cn/multi-cluster-manager/pkg/client/clientset/versioned/scheme"
	informers "harmonycloud.cn/multi-cluster-manager/pkg/client/informers/externalversions/multicluster/v1alpha1"
	lister "harmonycloud.cn/multi-cluster-manager/pkg/client/listers/multicluster/v1alpha1"
	"harmonycloud.cn/multi-cluster-manager/pkg/util/sliceutil"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	kubeclient "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	jsonpatch "github.com/evanphx/json-patch"
	corev1 "k8s.io/api/core/v1"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

const (
	controllerName                = "resourceBindingController"
	ManagerNamespace              = ""
	FinalizerName                 = ""
	ResourceBindingLabelName      = "multicluster.harmonycloud.cn.ResourceBinding"
	ResourceGvkLabelName          = "multicluster.harmonycloud.cn.ResourceGvk"
	MultiClusterResourceLabelName = "multicluster.harmonycloud.cn.MultiClusterResource"
)

var resourceBindingLog = logf.Log.WithName(controllerName)

type ResourceBindingController struct {
	// crd resource client
	managerClientSet clientset.Interface
	//
	syncResourceBinding func(ctx context.Context, key string) error
	//
	resourceBindingLister lister.MultiClusterResourceBindingLister
	resourceBindingSynced cache.InformerSynced
	//
	multiClusterResourceLister lister.MultiClusterResourceLister
	multiClusterResourceSynced cache.InformerSynced
	//
	clusterResourceLister lister.ClusterResourceLister
	clusterResourceSynced cache.InformerSynced
	//
	workqueue workqueue.RateLimitingInterface
	//
	recorder record.EventRecorder
}

func NewResourceBindingController(
	managerClientSet clientset.Interface,
	resourceBindingInformer informers.MultiClusterResourceBindingInformer,
	multiClusterResourceInformer informers.MultiClusterResourceInformer,
	clusterResourceInformer informers.ClusterResourceInformer,
	config *rest.Config) *ResourceBindingController {

	kubeClient := kubeclient.NewForConfigOrDie(config)

	// Add managerScheme types to the default Kubernetes Scheme so Events can be
	utilruntime.Must(managerScheme.AddToScheme(scheme.Scheme))

	resourceBindingLog.V(4).Info("Creating resourceBinding event broadcaster")
	eventbroadcaster := record.NewBroadcaster()
	eventbroadcaster.StartStructuredLogging(0)
	eventbroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{
		Interface: kubeClient.CoreV1().Events(""),
	})

	recorder := eventbroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerName})

	controller := &ResourceBindingController{
		managerClientSet:           managerClientSet,
		resourceBindingLister:      resourceBindingInformer.Lister(),
		resourceBindingSynced:      resourceBindingInformer.Informer().HasSynced,
		multiClusterResourceLister: multiClusterResourceInformer.Lister(),
		multiClusterResourceSynced: multiClusterResourceInformer.Informer().HasSynced,
		clusterResourceLister:      clusterResourceInformer.Lister(),
		clusterResourceSynced:      clusterResourceInformer.Informer().HasSynced,
		workqueue:                  workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "MultiClusterResourceBinding"),
		recorder:                   recorder,
	}

	controller.syncResourceBinding = controller.syncHandler

	resourceBindingLog.Info("setting up event handles")

	resourceBindingInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.bindingEventHandle,
		UpdateFunc: func(oldObj, newObj interface{}) {
			if equalSpec(oldObj, newObj) {
				return
			}
			controller.bindingEventHandle(newObj)
		},
		DeleteFunc: controller.bindingEventHandle,
	})

	clusterResourceInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: controller.updateStatusClusterResourceEventHandler,
		DeleteFunc: controller.deleteClusterResourceEventHandler,
	})
	// add event when multiClusterResource spec changed
	multiClusterResourceInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: controller.updateMultiClusterResourceEventHandler,
	})

	return controller
}

// EventHandler
func (c *ResourceBindingController) updateMultiClusterResourceEventHandler(old interface{}, cur interface{}) {
	// spec changed
	if equalSpec(old, cur) {
		return
	}
	multiClusterResource, ok := cur.(*v1alpha1.MultiClusterResource)
	if !ok {
		return
	}
	bindingList := c.getBindingForMultiClusterResource(multiClusterResource)
	if bindingList == nil {
		return
	}
	for _, binding := range bindingList {
		c.enqueue(binding)
	}
}

func (c *ResourceBindingController) updateStatusClusterResourceEventHandler(old, cur interface{}) {
	oldClusterResource, ok := old.(*v1alpha1.ClusterResource)
	if !ok {
		return
	}
	curClusterResource, ok := cur.(*v1alpha1.ClusterResource)
	if !ok {
		return
	}
	if oldClusterResource.Status == curClusterResource.Status {
		return
	}
	binding := c.getBindingForClusterResource(curClusterResource)
	if binding != nil {
		c.enqueue(binding)
	}
}

func (c *ResourceBindingController) deleteClusterResourceEventHandler(obj interface{}) {
	clusterResource, ok := obj.(*v1alpha1.ClusterResource)
	if !ok {
		return
	}
	binding := c.getBindingForClusterResource(clusterResource)
	if binding != nil {
		c.enqueue(binding)
	}
}

func (c *ResourceBindingController) bindingEventHandle(obj interface{}) {
	binding, ok := obj.(*v1alpha1.MultiClusterResourceBinding)
	if !ok {
		return
	}
	c.enqueue(binding)
}

//
func (c *ResourceBindingController) enqueue(rb *v1alpha1.MultiClusterResourceBinding) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(rb)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", rb, err))
		return
	}
	c.workqueue.Add(key)
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *ResourceBindingController) Run(ctx context.Context, workers int) {
	defer utilruntime.HandleCrash()
	defer c.workqueue.ShutDown()

	resourceBindingLog.Info("Starting resourceBindingController")

	resourceBindingLog.Info("Waiting for informer caches to sync")

	if !cache.WaitForNamedCacheSync(controllerName, ctx.Done(), c.clusterResourceSynced, c.multiClusterResourceSynced, c.resourceBindingSynced) {
		return
	}

	resourceBindingLog.Info("Starting workers")

	for i := 0; i < workers; i++ {
		go wait.UntilWithContext(ctx, c.runWorker, time.Second)
	}

	<-ctx.Done()
}

func (c *ResourceBindingController) runWorker(ctx context.Context) {
	for c.processNextWorkItem(ctx) {

	}
}

func (c *ResourceBindingController) processNextWorkItem(ctx context.Context) bool {
	obj, shutDown := c.workqueue.Get()
	if shutDown {
		return false
	}
	defer c.workqueue.Done(obj)

	err := c.syncResourceBinding(ctx, obj.(string))
	c.handleError(err, obj)

	return true
}

func (c *ResourceBindingController) handleError(err error, key interface{}) {
	if err == nil {
		c.workqueue.Forget(key)
		return
	}
	_, _, keyErr := cache.SplitMetaNamespaceKey(key.(string))
	if keyErr != nil {
		resourceBindingLog.Error(err, "Failed to split meta namespace cache key", "cacheKey", key)
	}

	utilruntime.HandleError(err)
	c.workqueue.Forget(key)
}

func (c *ResourceBindingController) syncHandler(ctx context.Context, key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	// get
	resourceBinding, err := c.resourceBindingLister.MultiClusterResourceBindings(namespace).Get(name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			utilruntime.HandleError(fmt.Errorf("foo '%s' in work queue no longer exists", key))
			return nil
		}
		return err
	}

	// add finalizer filed
	if resourceBinding.ObjectMeta.DeletionTimestamp.IsZero() && !sliceutil.ContainsString(resourceBinding.ObjectMeta.Finalizers, FinalizerName) {
		resourceBinding.ObjectMeta.Finalizers = append(resourceBinding.ObjectMeta.Finalizers, FinalizerName)
		resourceBinding, err = c.managerClientSet.MulticlusterV1alpha1().MultiClusterResourceBindings(namespace).Update(ctx, resourceBinding, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}

	// The object is being deleted
	if !resourceBinding.ObjectMeta.DeletionTimestamp.IsZero() && sliceutil.ContainsString(resourceBinding.ObjectMeta.Finalizers, FinalizerName) {
		resourceBinding.ObjectMeta.Finalizers = sliceutil.RemoveString(resourceBinding.ObjectMeta.Finalizers, FinalizerName)
		_, err = c.managerClientSet.MulticlusterV1alpha1().MultiClusterResourceBindings(namespace).Update(ctx, resourceBinding, metav1.UpdateOptions{})
		return err
	}

	// add labels
	newLabels := map[string]string{}
	for _, resource := range resourceBinding.Spec.Resources {
		multiClusterResource := c.getMultiClusterResource(resource.Name)
		if multiClusterResource != nil {
			labelKey := MultiClusterResourceLabelName + "." + multiClusterResource.GetName()
			newLabels[labelKey] = "1"
		}
	}
	if !labels.Equals(newLabels, resourceBinding.GetLabels()) {
		resourceBinding.SetLabels(newLabels)
		resourceBinding, err = c.managerClientSet.MulticlusterV1alpha1().MultiClusterResourceBindings(namespace).Update(ctx, resourceBinding, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}

	// create/update/delete ClusterResource
	err = c.syncClusterResource(resourceBinding)
	if err != nil {
		return err
	}

	return nil
}

// syncClusterResource create、update、delete ClusterResource, update binding status
func (c *ResourceBindingController) syncClusterResource(binding *v1alpha1.MultiClusterResourceBinding) error {
	if len(binding.Spec.Resources) == 0 {
		return errors.New("resources is empty")
	}
	owner := metav1.NewControllerRef(binding, binding.GroupVersionKind())
	if owner == nil {
		return errors.New("get owner fail")
	}
	//
	clusterResourceMap, err := c.getClusterResourceListForBinding(binding)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
	}
	for _, resource := range binding.Spec.Resources {
		if resource.Clusters != nil {
			for _, cluster := range resource.Clusters {
				mcr := c.getMultiClusterResource(resource.Name)
				if mcr == nil || mcr.Spec.Resource == nil || mcr.Spec.ResourceRef == nil {
					continue
				}
				clusterNamespace := getClusterNamespace(cluster.Name)
				//
				clusterResourceName := getClusterResourceName(binding.GetName(), mcr.Spec.ResourceRef)

				key := clusterNamespace + "-" + clusterResourceName
				clusterResource, ok := clusterResourceMap[key]
				if !ok {
					// create
					newClusterResource := &v1alpha1.ClusterResource{}
					newClusterResource.SetName(clusterResourceName)
					newClusterResource.SetNamespace(clusterNamespace)
					// labels
					newLabels := map[string]string{}
					newLabels[ResourceBindingLabelName] = binding.GetName()
					//
					newLabels[ResourceGvkLabelName] = getGvkLabelString(mcr.Spec.ResourceRef)
					newClusterResource.SetLabels(newLabels)
					// OwnerReferences
					newClusterResource.SetOwnerReferences([]metav1.OwnerReference{*owner})
					// resourceInfo
					if cluster.Override != nil {
						resourceInfo, err := ApplyJsonPatch(mcr.Spec.Resource, cluster.Override)
						if err == nil {
							newClusterResource.Spec.Resource = resourceInfo
						}
					} else {
						newClusterResource.Spec.Resource = mcr.Spec.Resource
					}
					// create clusterResource
					_, err = c.managerClientSet.MulticlusterV1alpha1().ClusterResources(clusterNamespace).Create(context.TODO(), newClusterResource, metav1.CreateOptions{})
					if err != nil {
						return err
					}
				} else {
					// update
					if len(clusterResource.Status.Phase) > 0 {
						var resourceStatus *common.MultiClusterResourceClusterStatus
						for _, item := range binding.Status.ClusterStatus {
							if cluster.Name == item.Name {
								// delete
								binding.Status.ClusterStatus = removeItemForClusterStatus(binding.Status.ClusterStatus, item)
							}
						}
						resourceStatus = &common.MultiClusterResourceClusterStatus{
							Name:                      cluster.Name,
							Resource:                  clusterResource.Name,
							ObservedReceiveGeneration: clusterResource.Status.ObservedReceiveGeneration,
							Phase:                     clusterResource.Status.Phase,
							Message:                   clusterResource.Status.Message,
							Binding:                   binding.Name,
						}
						binding.Status.ClusterStatus = append(binding.Status.ClusterStatus, *resourceStatus)
					}
					// resourceInfo
					resourceInfo := mcr.Spec.Resource
					if cluster.Override != nil {
						rInfo, err := ApplyJsonPatch(mcr.Spec.Resource, cluster.Override)
						if err == nil {
							resourceInfo = rInfo
						}
					}
					if string(clusterResource.Spec.Resource.Raw) != string(resourceInfo.Raw) {
						clusterResource.Spec.Resource = resourceInfo
						// labels
						newLabels := clusterResource.GetLabels()
						newLabels[ResourceBindingLabelName] = binding.GetName()
						//
						newLabels[ResourceGvkLabelName] = getGvkLabelString(mcr.Spec.ResourceRef)
						clusterResource.SetLabels(newLabels)
						// OwnerReferences
						clusterResource.SetOwnerReferences([]metav1.OwnerReference{*owner})
						// update
						_, err = c.managerClientSet.MulticlusterV1alpha1().ClusterResources(clusterNamespace).Update(context.TODO(), clusterResource, metav1.UpdateOptions{})
						if err != nil {
							return err
						}
					}
					delete(clusterResourceMap, key)
				}
				// delete
				if len(clusterResourceMap) > 0 {
					for _, r := range clusterResourceMap {
						err = c.managerClientSet.MulticlusterV1alpha1().ClusterResources(clusterNamespace).Delete(context.TODO(), r.GetName(), metav1.DeleteOptions{})
						if err != nil {
							return err
						}
					}
				}
			}
		}
	}
	if len(clusterResourceMap) > 0 && len(binding.Status.ClusterStatus) > 0 {
		// updateStatus
		binding, err = c.managerClientSet.MulticlusterV1alpha1().MultiClusterResourceBindings(binding.Namespace).UpdateStatus(context.TODO(), binding, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}

// util
func getClusterNamespace(clusterName string) string {
	return clusterName
}

//
func (c *ResourceBindingController) getBindingForMultiClusterResource(multiClusterResource *v1alpha1.MultiClusterResource) []*v1alpha1.MultiClusterResourceBinding {
	selector, err := labels.Parse(MultiClusterResourceLabelName + "." + multiClusterResource.Name + "=1")
	if err != nil {
		return nil
	}
	bindingList, err := c.resourceBindingLister.List(selector)
	if err != nil {
		return nil
	}
	return bindingList
}

//
func (c *ResourceBindingController) getBindingForClusterResource(clusterResource *v1alpha1.ClusterResource) *v1alpha1.MultiClusterResourceBinding {
	controllerRef := metav1.GetControllerOf(clusterResource)
	if controllerRef == nil {
		return nil
	}
	return c.resolveControllerRef(controllerRef)
}

func (c *ResourceBindingController) resolveControllerRef(controllerRef *metav1.OwnerReference) *v1alpha1.MultiClusterResourceBinding {
	if controllerRef.Kind != "MultiClusterResourceBinding" {
		return nil
	}

	binding, err := c.resourceBindingLister.MultiClusterResourceBindings(ManagerNamespace).Get(controllerRef.Name)
	if err != nil {
		return nil
	}
	if binding.UID != controllerRef.UID {
		return nil
	}
	return binding
}

//
func (c *ResourceBindingController) getMultiClusterResource(name string) *v1alpha1.MultiClusterResource {
	mcr, err := c.multiClusterResourceLister.MultiClusterResources(ManagerNamespace).Get(name)
	if err != nil {
		return nil
	}
	return mcr
}

func ApplyJsonPatch(resource *runtime.RawExtension, override []common.JSONPatch) (*runtime.RawExtension, error) {
	jsonPatchBytes, err := json.Marshal(override)
	if err != nil {
		return nil, err
	}
	patch, err := jsonpatch.DecodePatch(jsonPatchBytes)
	if err != nil {
		return nil, err
	}
	patchedObjectJsonBytes, err := patch.Apply(resource.Raw)
	if err != nil {
		return nil, err
	}
	return &runtime.RawExtension{Raw: patchedObjectJsonBytes}, nil
}

func getClusterResourceName(bindingName string, gvk *schema.GroupVersionKind) string {
	gvkString := getGvkLabelString(gvk)
	return bindingName + gvkString
}

func getGvkLabelString(gvk *schema.GroupVersionKind) string {
	gvkString := gvk.Group + ":" + gvk.Version + ":" + gvk.Kind
	if len(gvk.Group) == 0 {
		gvkString = gvk.Version + ":" + gvk.Kind
	}
	return gvkString
}

// getClusterResourceListForBinding go through ResourceBinding to find clusterResource list
// clusterResource list change to clusterResource map, map key:<resourceNamespace>-<resourceName>
func (c *ResourceBindingController) getClusterResourceListForBinding(binding *v1alpha1.MultiClusterResourceBinding) (map[string]*v1alpha1.ClusterResource, error) {
	if len(binding.GetName()) <= 0 {
		return nil, errors.New("binding name is empty")
	}
	resourceMap := map[string]*v1alpha1.ClusterResource{}
	selector, _ := labels.Parse(ResourceBindingLabelName + "=" + binding.GetName())

	resourceList, err := c.clusterResourceLister.List(selector)
	if err != nil {
		return resourceMap, err
	}

	for _, resource := range resourceList {
		key := resource.GetNamespace() + "-" + resource.GetName()
		resourceMap[key] = resource
	}
	return resourceMap, nil
}

//
func equalSpec(obj1, obj2 interface{}) bool {
	return resourceSpec(obj1) == resourceSpec(obj2)
}
func resourceSpec(obj interface{}) string {
	resource, ok := reflect.ValueOf(obj).Interface().(*unstructured.Unstructured)
	if ok {
		specObj, ok := resource.Object["spec"]
		if ok {
			spec, ok := reflect.ValueOf(specObj).Interface().(map[string]interface{})
			if ok {
				specData, err := json.Marshal(spec)
				if err != nil {
					return ""
				}
				return string(specData)
			}
		}
	}
	return ""
}

func removeItemForClusterStatus(itemList []common.MultiClusterResourceClusterStatus, item common.MultiClusterResourceClusterStatus) []common.MultiClusterResourceClusterStatus {
	index := GetIndexWithObject(itemList, item)
	if index < 0 {
		return itemList
	}
	return RemoveObjectWithIndex(itemList, index)
}

func GetIndexWithObject(slice []common.MultiClusterResourceClusterStatus, obj common.MultiClusterResourceClusterStatus) int {
	if len(slice) == 0 {
		return -1
	}
	index := -1
	for i := 0; i < len(slice); i++ {
		if slice[i] == obj {
			index = i
			break
		}
	}
	return index
}

func RemoveObjectWithIndex(array []common.MultiClusterResourceClusterStatus, index int) []common.MultiClusterResourceClusterStatus {
	if index == 0 {
		return array[1:]
	} else if index == 1 {
		return append(array[index+1:], array[0])
	} else if index >= len(array) {
		return array
	} else if index == len(array)-1 {
		return array[0:index]
	} else if index == len(array)-2 {
		return append(array[:index], array[index+1])
	}
	return append(array[:index], array[index+1:]...)
}
