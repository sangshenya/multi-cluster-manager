package resource_binding

import (
	"context"
	"fmt"

	"harmonycloud.cn/stellaris/pkg/apis/multicluster/common"

	"github.com/go-logr/logr"
	"harmonycloud.cn/stellaris/pkg/apis/multicluster/v1alpha1"
	managerCommon "harmonycloud.cn/stellaris/pkg/common"
	controllerCommon "harmonycloud.cn/stellaris/pkg/controller/common"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}
		return ctrl.Result{}, err
	}

	// add Finalizers
	if controllerCommon.ShouldAddFinalizer(instance) {
		err = controllerCommon.AddFinalizer(ctx, r.Client, instance)
		if err != nil {
			r.log.Error(err, fmt.Sprintf("append finalizer filed, resource(%s)", instance.Name))
			return controllerCommon.ReQueueResult(err)
		}
		return ctrl.Result{}, nil
	}

	// the object is being deleted
	if !instance.GetDeletionTimestamp().IsZero() {
		err = controllerCommon.RemoveFinalizer(ctx, r.Client, instance)
		if err != nil {
			r.log.Error(err, fmt.Sprintf("delete finalizer filed, resource(%s)", instance.Name))
			return controllerCommon.ReQueueResult(err)
		}
		return ctrl.Result{}, nil
	}

	// get ClusterResourceList
	clusterResourceList, err := getClusterResourceListForBinding(ctx, r.Client, instance)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			r.log.Error(err, fmt.Sprintf("get clusterResource for resource failed, resource(%s)", instance.Name))
			return controllerCommon.ReQueueResult(err)
		}
	}

	// sync ClusterResource
	err = syncClusterResource(ctx, r.Client, clusterResourceList, instance)
	if err != nil {
		r.log.Error(err, fmt.Sprintf("sync ClusterResource failed, resource(%s)", instance.Name))
		return controllerCommon.ReQueueResult(err)
	}

	// update status
	err = updateBindingStatus(ctx, r.Client, instance, clusterResourceList)
	if err != nil {
		r.log.Error(err, fmt.Sprintf("update binding status failed, resource(%s)", instance.Name))
		return controllerCommon.ReQueueResult(err)
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

func getMultiClusterResourceForName(ctx context.Context, clientSet client.Client, multiClusterResourceName string) (*v1alpha1.MultiClusterResource, error) {
	object := &v1alpha1.MultiClusterResource{}
	namespacedName := types.NamespacedName{
		Namespace: managerCommon.ManagerNamespace,
		Name:      multiClusterResourceName,
	}
	err := clientSet.Get(ctx, namespacedName, object)
	return object, err
}

func updateBindingStatus(ctx context.Context, clientSet client.Client, binding *v1alpha1.MultiClusterResourceBinding, clusterResourceList *v1alpha1.ClusterResourceList) error {
	updateStatus := false
	for _, clusterResource := range clusterResourceList.Items {
		// no status
		if len(clusterResource.Status.Phase) <= 0 {
			continue
		}

		bindingStatusMap := bindingClusterStatusMap(binding)
		// find target multiClusterResourceClusterStatus
		key := bindingClusterStatusMapKey(managerCommon.ClusterName(clusterResource.GetNamespace()), clusterResource.GetName())
		bindingStatus, ok := bindingStatusMap[key]
		if ok {
			if statusEqual(clusterResource.Status, *bindingStatus) {
				continue
			}
			binding.Status.ClusterStatus = removeItemForClusterStatusList(binding.Status.ClusterStatus, *bindingStatus)
		}
		// should update binding status
		updateStatus = true
		// new resourceStatus
		multiClusterResourceClusterStatus := common.MultiClusterResourceClusterStatus{
			Name:                      managerCommon.ClusterName(clusterResource.GetNamespace()),
			Resource:                  clusterResource.Name,
			ObservedReceiveGeneration: clusterResource.Status.ObservedReceiveGeneration,
			Phase:                     clusterResource.Status.Phase,
			Message:                   clusterResource.Status.Message,
			Binding:                   binding.Name,
		}
		binding.Status.ClusterStatus = append(binding.Status.ClusterStatus, multiClusterResourceClusterStatus)
	}
	if updateStatus {
		// update binding status
		return clientSet.Status().Update(ctx, binding)
	}
	return nil
}

func bindingClusterStatusMap(binding *v1alpha1.MultiClusterResourceBinding) map[string]*common.MultiClusterResourceClusterStatus {
	statusMap := map[string]*common.MultiClusterResourceClusterStatus{}
	for _, item := range binding.Status.ClusterStatus {
		statusKey := bindingClusterStatusMapKey(item.Name, item.Resource)
		statusMap[statusKey] = &item
	}
	return statusMap
}

func bindingClusterStatusMapKey(clusterName, resourceName string) string {
	return clusterName + ":" + resourceName
}

func statusEqual(clusterResourceStatus v1alpha1.ClusterResourceStatus, bindingStatus common.MultiClusterResourceClusterStatus) bool {
	if clusterResourceStatus.Phase != bindingStatus.Phase || clusterResourceStatus.Message != bindingStatus.Message || clusterResourceStatus.ObservedReceiveGeneration != bindingStatus.ObservedReceiveGeneration {
		return false
	}
	return true
}
