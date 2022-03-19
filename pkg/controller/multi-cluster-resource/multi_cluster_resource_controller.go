package multi_cluster_resource

import (
	"context"
	"errors"
	"fmt"
	"time"

	"k8s.io/client-go/tools/record"

	resource_binding "harmonycloud.cn/stellaris/pkg/controller/resource-binding"

	managerCommon "harmonycloud.cn/stellaris/pkg/common"
	sliceutils "harmonycloud.cn/stellaris/pkg/utils/slice"

	"github.com/go-logr/logr"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	controllerCommon "harmonycloud.cn/stellaris/pkg/controller/common"

	"harmonycloud.cn/stellaris/pkg/apis/multicluster/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Reconciler struct {
	client.Client
	log      logr.Logger
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

func (r *Reconciler) Reconcile(ctx context.Context, request ctrl.Request) (ctrl.Result, error) {
	r.log.V(4).Info(fmt.Sprintf("Start Reconciling MultiClusterResource(%s:%s)", request.Namespace, request.Name))
	defer r.log.V(4).Info(fmt.Sprintf("End Reconciling MultiClusterResource(%s:%s)", request.Namespace, request.Name))

	// get resource
	instance := &v1alpha1.MultiClusterResource{}
	err := r.Client.Get(ctx, request.NamespacedName, instance)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// add Finalizers
	if controllerCommon.ShouldAddFinalizer(instance) {
		if err = controllerCommon.AddFinalizer(ctx, r.Client, instance); err != nil {
			r.log.Error(err, fmt.Sprintf("append finalizer failed, resource(%s:%s)", instance.Namespace, instance.Name))
			r.Recorder.Event(instance, "Warning", "FailedAddFinalizers", err.Error())
			return ctrl.Result{Requeue: true, RequeueAfter: 5 * time.Second}, err
		}
		return ctrl.Result{}, nil
	}

	// the object is being deleted
	if !instance.GetDeletionTimestamp().IsZero() {
		if err = r.updateBinding(ctx, instance); err != nil {
			r.log.Error(err, fmt.Sprintf("delete finalizer plan failed, resource(%s:%s)", instance.Namespace, instance.Name))
			r.Recorder.Event(instance, "Warning", "FailedDeleteFinalizersPlan", err.Error())
			return ctrl.Result{Requeue: true, RequeueAfter: 5 * time.Second}, err
		}
		if err = controllerCommon.RemoveFinalizer(ctx, r.Client, instance); err != nil {
			r.log.Error(err, fmt.Sprintf("delete finalizer failed, resource(%s:%s)", instance.Namespace, instance.Name))
			r.Recorder.Event(instance, "Warning", "FailedDeleteFinalizers", err.Error())
			return ctrl.Result{Requeue: true, RequeueAfter: 5 * time.Second}, err
		}
		return ctrl.Result{}, nil
	}
	// find binding list, then sync clusterResource
	if err = r.syncBindingAndClusterResource(ctx, instance); err != nil {
		r.log.Error(err, fmt.Sprintf("sync binding and clusterResource failed, resource(%s:%s)", instance.Namespace, instance.Name))
		r.Recorder.Event(instance, "Warning", "FailedSyncMultiClusterResource", err.Error())
		return ctrl.Result{Requeue: true, RequeueAfter: 5 * time.Second}, err
	}
	return ctrl.Result{}, nil
}

func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.MultiClusterResource{}).
		Complete(r)
}

func Setup(mgr ctrl.Manager, controllerCommon controllerCommon.Args) error {
	reconciler := Reconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
		log:    logf.Log.WithName("multi_cluster_resource_controller"),
	}
	reconciler.Recorder = mgr.GetEventRecorderFor("stellaris-core")
	return reconciler.SetupWithManager(mgr)
}

// syncBindingAndClusterResource get binding list, and sync clusterResource
func (r *Reconciler) syncBindingAndClusterResource(ctx context.Context, multiClusterResource *v1alpha1.MultiClusterResource) error {
	bindingList, err := getMultiClusterResourceBindingList(ctx, r.Client, multiClusterResource)
	if err != nil {
		return errors.New("get multiClusterResourceBindingList failed," + err.Error())
	}

	for _, binding := range bindingList.Items {
		err = resource_binding.SyncClusterResourceWithBinding(ctx, r.Client, &binding)
		if err != nil {
			return errors.New("sync ClusterResource failed," + err.Error())
		}
	}
	return nil
}

// updateBinding update or delete binding when multiClusterResource deleted
func (r *Reconciler) updateBinding(ctx context.Context, multiClusterResource *v1alpha1.MultiClusterResource) error {
	bindingList, err := getMultiClusterResourceBindingList(ctx, r.Client, multiClusterResource)
	if err != nil {
		return errors.New("get multiClusterResourceBindingList failed, " + err.Error())
	}

	for _, binding := range bindingList.Items {
		// edit binding
		for _, resource := range binding.Spec.Resources {
			if resource.Name == multiClusterResource.GetName() {
				binding.Spec.Resources = removeResource(binding.Spec.Resources, resource)
			}
		}

		if len(binding.Spec.Resources) == 0 {
			// delete
			err = r.Client.Delete(ctx, &binding)
			if err != nil {
				return errors.New("delete binding failed," + err.Error())
			}
			continue
		}
		// update
		err = r.Client.Update(ctx, &binding)
		if err != nil {
			return errors.New("update binding failed," + err.Error())
		}
	}
	return nil
}

func getMultiClusterResourceBindingList(
	ctx context.Context,
	clientSet client.Client,
	multiClusterResource *v1alpha1.MultiClusterResource) (*v1alpha1.MultiClusterResourceBindingList, error) {
	selector, err := managerCommon.GetMultiClusterResourceSelector(multiClusterResource.GetName())
	if err != nil {
		return nil, err
	}
	bindingList := &v1alpha1.MultiClusterResourceBindingList{}
	err = clientSet.List(ctx, bindingList, &client.ListOptions{
		LabelSelector: selector,
	})
	return bindingList, err
}

func removeResource(
	resources []v1alpha1.MultiClusterResourceBindingResource,
	resource v1alpha1.MultiClusterResourceBindingResource) []v1alpha1.MultiClusterResourceBindingResource {
	if len(resources) == 0 {
		return resources
	}
	var objectList []interface{}
	for _, items := range resources {
		objectList = append(objectList, items)
	}
	index := sliceutils.GetIndexWithObject(objectList, resource)
	list := sliceutils.RemoveObjectWithIndex(objectList, index)
	if len(list) == 0 {
		return []v1alpha1.MultiClusterResourceBindingResource{}
	}
	var resourceList []v1alpha1.MultiClusterResourceBindingResource
	for _, obj := range list {
		if res, ok := obj.(v1alpha1.MultiClusterResourceBindingResource); ok {
			resourceList = append(resourceList, res)
		}
	}
	return resourceList
}
