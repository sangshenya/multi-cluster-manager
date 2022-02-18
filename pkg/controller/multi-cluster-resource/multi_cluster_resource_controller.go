package multi_cluster_resource

import (
	"context"
	"fmt"

	resource_binding "harmonycloud.cn/stellaris/pkg/controller/resource-binding"

	managerCommon "harmonycloud.cn/stellaris/pkg/common"
	sliceutil "harmonycloud.cn/stellaris/pkg/util/slice"

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
	log    logr.Logger
	Scheme *runtime.Scheme
}

func (r *Reconciler) Reconcile(ctx context.Context, request ctrl.Request) (ctrl.Result, error) {
	r.log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	r.log.Info("Reconciling MultiClusterResource")

	// get resource
	instance := &v1alpha1.MultiClusterResource{}
	err := r.Client.Get(ctx, request.NamespacedName, instance)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// add Finalizers
	if controllerCommon.ShouldAddFinalizer(instance) {
		return r.addFinalizer(ctx, instance)
	}

	// the object is being deleted
	if !instance.GetDeletionTimestamp().IsZero() {
		return r.removeMultiClusterResource(ctx, instance)
	}
	// find binding list, then sync clusterResource
	return r.syncBindingAndClusterResource(ctx, instance)
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
	return reconciler.SetupWithManager(mgr)
}

// syncBindingAndClusterResource get binding list, and sync clusterResource
func (r *Reconciler) syncBindingAndClusterResource(ctx context.Context, multiClusterResource *v1alpha1.MultiClusterResource) (ctrl.Result, error) {
	bindingList, err := getMultiClusterResourceBindingListForMultiClusterResource(ctx, r.Client, multiClusterResource)
	if err != nil {
		r.log.Error(err, fmt.Sprintf("get multiClusterResourceBindingList failed, resource(%s)", multiClusterResource.Name))
		return controllerCommon.ReQueueResult(err)
	}

	for _, binding := range bindingList.Items {
		err = resource_binding.SyncClusterResourceWithBinding(ctx, r.Client, &binding)
		if err != nil {
			r.log.Error(err, fmt.Sprintf("sync ClusterResource failed, resource(%s)", multiClusterResource.Name))
			return controllerCommon.ReQueueResult(err)
		}
	}
	return ctrl.Result{}, nil
}

// updateBinding update or delete binding when multiClusterResource deleted
func (r *Reconciler) updateBinding(ctx context.Context, multiClusterResource *v1alpha1.MultiClusterResource) error {
	bindingList, err := getMultiClusterResourceBindingListForMultiClusterResource(ctx, r.Client, multiClusterResource)
	if err != nil {
		r.log.Error(err, fmt.Sprintf("get multiClusterResourceBindingList failed, resource(%s)", multiClusterResource.Name))
		return err
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
				r.log.Error(err, fmt.Sprintf("delete binding failed, resource(%s)", multiClusterResource.GetName()))
				return err
			}
			continue
		}
		// update
		err = r.Client.Update(ctx, &binding)
		if err != nil {
			r.log.Error(err, fmt.Sprintf("update binding failed, resource(%s)", multiClusterResource.GetName()))
			return err
		}
	}
	return nil
}

func (r *Reconciler) addFinalizer(ctx context.Context, instance *v1alpha1.MultiClusterResource) (ctrl.Result, error) {
	err := controllerCommon.AddFinalizer(ctx, r.Client, instance)
	if err != nil {
		r.log.Error(err, fmt.Sprintf("append finalizer filed, resource(%s)", instance.Name))
		return controllerCommon.ReQueueResult(err)
	}
	return ctrl.Result{}, nil
}

func (r *Reconciler) removeMultiClusterResource(ctx context.Context, instance *v1alpha1.MultiClusterResource) (ctrl.Result, error) {
	// edit bindings
	err := r.updateBinding(ctx, instance)
	if err != nil {
		return controllerCommon.ReQueueResult(err)
	}
	return r.removeFinalizer(ctx, instance)
}

func (r *Reconciler) removeFinalizer(ctx context.Context, instance *v1alpha1.MultiClusterResource) (ctrl.Result, error) {
	err := controllerCommon.RemoveFinalizer(ctx, r.Client, instance)
	if err != nil {
		r.log.Error(err, fmt.Sprintf("delete finalizer filed, resource(%s)", instance.Name))
		return controllerCommon.ReQueueResult(err)
	}
	return ctrl.Result{}, nil
}

func getMultiClusterResourceBindingListForMultiClusterResource(ctx context.Context, clientSet client.Client, multiClusterResource *v1alpha1.MultiClusterResource) (*v1alpha1.MultiClusterResourceBindingList, error) {
	selector, err := managerCommon.GetMultiClusterResourceSelectorForMultiClusterResourceName(multiClusterResource.GetName())
	if err != nil {
		return nil, err
	}
	bindingList := &v1alpha1.MultiClusterResourceBindingList{}
	err = clientSet.List(ctx, bindingList, &client.ListOptions{
		LabelSelector: selector,
	})
	return bindingList, err
}

func removeResource(resources []v1alpha1.MultiClusterResourceBindingResource, resource v1alpha1.MultiClusterResourceBindingResource) []v1alpha1.MultiClusterResourceBindingResource {
	if len(resources) == 0 {
		return resources
	}
	var objectList []interface{}
	for _, items := range resources {
		objectList = append(objectList, items)
	}
	index := sliceutil.GetIndexWithObject(objectList, resource)
	list := sliceutil.RemoveObjectWithIndex(objectList, index)
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
