package resource_binding

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"harmonycloud.cn/stellaris/pkg/apis/multicluster/v1alpha1"
	managerCommon "harmonycloud.cn/stellaris/pkg/common"
	controllerCommon "harmonycloud.cn/stellaris/pkg/controller/common"
	sliceutil "harmonycloud.cn/stellaris/pkg/util/slice"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

type Reconciler struct {
	client.Client
	log    logr.Logger
	Scheme *runtime.Scheme
}

func (r *Reconciler) Reconcile(ctx context.Context, request ctrl.Request) (ctrl.Result, error) {
	r.log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	r.log.Info("Reconciling MultiClusterResourceBinding")
	// get resource
	instance := &v1alpha1.MultiClusterResourceBinding{}
	err := r.Client.Get(ctx, request.NamespacedName, instance)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// add Finalizers
	if instance.ObjectMeta.DeletionTimestamp.IsZero() && !sliceutil.ContainsString(instance.ObjectMeta.Finalizers, managerCommon.FinalizerName) {
		instance.ObjectMeta.Finalizers = append(instance.ObjectMeta.Finalizers, managerCommon.FinalizerName)
		if err = r.Client.Update(ctx, instance); err != nil {
			return ctrl.Result{
				Requeue:      true,
				RequeueAfter: 30 * time.Second,
			}, fmt.Errorf("append finalizer filed to resource %s failed: %s", instance.Name, err)
		}
		return ctrl.Result{}, nil
	}

	// the object is being deleted
	if !instance.ObjectMeta.DeletionTimestamp.IsZero() && sliceutil.ContainsString(instance.ObjectMeta.Finalizers, managerCommon.FinalizerName) {
		instance.ObjectMeta.Finalizers = sliceutil.RemoveString(instance.ObjectMeta.Finalizers, managerCommon.FinalizerName)
		if err = r.Client.Update(ctx, instance); err != nil {
			return ctrl.Result{
				Requeue:      true,
				RequeueAfter: 30 * time.Second,
			}, fmt.Errorf("delete finalizer filed from resource %s failed: %s", instance.Name, err)
		}
		return ctrl.Result{}, nil
	}

	// add labels
	newLabels := r.getLabelsWithBinding(instance)
	if !labels.Equals(newLabels, instance.GetLabels()) {
		instance.SetLabels(newLabels)
		if err = r.Client.Update(ctx, instance); err != nil {
			return ctrl.Result{
				Requeue:      true,
				RequeueAfter: 30 * time.Second,
			}, fmt.Errorf("add labels filed resource %s failed: %s", instance.Name, err)
		}
		return ctrl.Result{}, nil
	}

	// sync ClusterResource
	err = syncClusterResource(r.Client, instance)
	if err != nil {
		return ctrl.Result{
			Requeue:      true,
			RequeueAfter: 30 * time.Second,
		}, fmt.Errorf("sync ClusterResource failed: %s,  resource: %s", err, instance.Name)
	}

	return ctrl.Result{}, nil
}

func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.MultiClusterResourceBinding{}).
		Complete(r)
}

func Setup(mgr ctrl.Manager, controllerCommon controllerCommon.Args) error {
	reconciler := Reconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
		log:    logf.Log.WithName("resource_binding_controller"),
	}
	return reconciler.SetupWithManager(mgr)
}

//
func getMultiClusterResourceForName(clientSet client.Client, multiClusterResourceName string) (*v1alpha1.MultiClusterResource, error) {
	object := &v1alpha1.MultiClusterResource{}
	namespacedName := types.NamespacedName{
		Namespace: managerCommon.ManagerNamespace,
		Name:      multiClusterResourceName,
	}
	err := clientSet.Get(context.TODO(), namespacedName, object)
	return object, err
}

// getClusterResourceListForBinding go through ResourceBinding to find clusterResource list
// clusterResource list change to clusterResource map, map key:<resourceNamespace>-<resourceName>
func getClusterResourceListForBinding(clientSet client.Client, binding *v1alpha1.MultiClusterResourceBinding) (map[string]*v1alpha1.ClusterResource, error) {
	if len(binding.GetName()) <= 0 {
		return nil, errors.New("binding name is empty")
	}
	resourceMap := map[string]*v1alpha1.ClusterResource{}
	selector, _ := labels.Parse(managerCommon.ResourceBindingLabelName + "=" + binding.GetName())

	resourceList := &v1alpha1.ClusterResourceList{}
	err := clientSet.List(context.TODO(), resourceList, &client.ListOptions{
		LabelSelector: selector,
	})
	if err != nil {
		return resourceMap, err
	}
	for _, resource := range resourceList.Items {
		key := mapKey(resource.GetNamespace(), resource.GetName())
		resourceMap[key] = &resource
	}
	return resourceMap, nil
}

func mapKey(resourceNamespace, resourceName string) string {
	return resourceNamespace + "-" + resourceName
}

func getClusterResourceName(bindingName string, gvk *metav1.GroupVersionKind) string {
	gvkString := managerCommon.GvkLabelString(gvk)
	return bindingName + gvkString
}

// getLabelsWithBinding create binding labels
// Walk through the MultiClusterResourceList associated with the MultiClusterResourceBinding then add MultiClusterResourceLabels
// e.g."multicluster.harmonycloud.cn.multiClusterResource.<multiClusterResourceName>" = "1"
// if binding`s OwnerReferences is MultiClusterResourceSchedulePolicy then add SchedulePolicyLabel
// e.g."multicluster.harmonycloud.cn.schedulePolicy" = <policyName>
func (r *Reconciler) getLabelsWithBinding(binding *v1alpha1.MultiClusterResourceBinding) map[string]string {
	bindingLabels := map[string]string{}
	// SchedulePolicyLabels
	controllerRef := metav1.GetControllerOf(binding)
	if controllerRef != nil && controllerRef.Kind == "MultiClusterResourceSchedulePolicy" {
		bindingLabels[managerCommon.MultiClusterResourceSchedulePolicyLabelName] = controllerRef.Name
	}
	// MultiClusterResourceLabels
	for _, resource := range binding.Spec.Resources {
		multiClusterResource, err := getMultiClusterResourceForName(r.Client, resource.Name)
		if err != nil {
			continue
		}
		if multiClusterResource != nil {
			labelKey := managerCommon.MultiClusterResourceLabelName + "." + multiClusterResource.GetName()
			bindingLabels[labelKey] = "1"
		}
	}
	return bindingLabels
}
