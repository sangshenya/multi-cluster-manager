package multi_cluster_resource

import (
	"context"
	"fmt"
	"time"

	"harmonycloud.cn/stellaris/pkg/controller/resource_binding"

	managerCommon "harmonycloud.cn/stellaris/pkg/common"
	sliceutil "harmonycloud.cn/stellaris/pkg/util/slice"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}
		return ctrl.Result{}, err
	}

	// add Finalizers
	if instance.ObjectMeta.DeletionTimestamp.IsZero() && !sliceutil.ContainsString(instance.ObjectMeta.Finalizers, managerCommon.FinalizerName) {
		instance.ObjectMeta.Finalizers = append(instance.ObjectMeta.Finalizers, managerCommon.FinalizerName)
		if err = r.Client.Update(ctx, instance); err != nil {
			r.log.Error(err, fmt.Sprintf("append finalizer filed to resource(%s) failed", instance.Name))
			return reQueueResult(err)
		}
		return ctrl.Result{}, nil
	}

	// the object is being deleted
	if !instance.ObjectMeta.DeletionTimestamp.IsZero() && sliceutil.ContainsString(instance.ObjectMeta.Finalizers, managerCommon.FinalizerName) {
		// edit bindings
		err = r.updateBinding(ctx, instance)
		if err != nil {
			return reQueueResult(err)
		}

		instance.ObjectMeta.Finalizers = sliceutil.RemoveString(instance.ObjectMeta.Finalizers, managerCommon.FinalizerName)
		if err = r.Client.Update(ctx, instance); err != nil {
			r.log.Error(err, fmt.Sprintf("delete finalizer filed from resource(%s) failed", instance.Name))
			return reQueueResult(err)

		}
		return ctrl.Result{}, nil
	}
	// find binding list, then sync clusterResource
	err = r.syncBindingAndClusterResource(ctx, instance)
	if err != nil {
		return reQueueResult(err)
	}

	return ctrl.Result{}, nil
}

func reQueueResult(err error) (ctrl.Result, error) {
	return ctrl.Result{
		Requeue:      true,
		RequeueAfter: 30 * time.Second,
	}, err
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
func (r *Reconciler) syncBindingAndClusterResource(ctx context.Context, multiClusterResource *v1alpha1.MultiClusterResource) error {
	bindingList, err := getMultiClusterResourceBindingListForMultiClusterResource(ctx, r.Client, multiClusterResource)
	if err != nil {
		r.log.Error(err, fmt.Sprintf("get multiClusterResourceBindingList failed, resource(%s)", multiClusterResource.Name))
		return err
	}

	for _, binding := range bindingList.Items {
		err = resource_binding.SyncClusterResourceWithBinding(ctx, r.Client, &binding)
		if err != nil {
			r.log.Error(err, fmt.Sprintf("sync ClusterResource failed, resource(%s)", multiClusterResource.Name))
			return err
		}
	}
	return nil
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
	var objectList []interface{}
	for _, items := range resources {
		objectList = append(objectList, items)
	}
	index := sliceutil.GetIndexWithObject(objectList, resource)
	list := sliceutil.RemoveObjectWithIndex(objectList, index)
	if len(list) <= 0 {
		return resources
	}
	var resourceList []v1alpha1.MultiClusterResourceBindingResource
	for _, obj := range list {
		if res, ok := obj.(v1alpha1.MultiClusterResourceBindingResource); ok {
			resourceList = append(resourceList, res)
		}
	}
	return resourceList
}
