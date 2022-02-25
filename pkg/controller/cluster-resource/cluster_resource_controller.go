package cluster_resource

import (
	"context"
	"fmt"
	"strings"

	managerCommon "harmonycloud.cn/stellaris/pkg/common"

	"harmonycloud.cn/stellaris/pkg/apis/multicluster/common"

	"github.com/go-logr/logr"
	"harmonycloud.cn/stellaris/pkg/apis/multicluster/v1alpha1"
	controllerCommon "harmonycloud.cn/stellaris/pkg/controller/common"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

type Reconciler struct {
	client.Client
	log            logr.Logger
	Scheme         *runtime.Scheme
	isControlPlane bool
}

func (r *Reconciler) Reconcile(ctx context.Context, request ctrl.Request) (ctrl.Result, error) {
	// core focus on clusterNamespaces ClusterResource
	if r.isControlPlane && !strings.HasPrefix(request.Namespace, managerCommon.ClusterWorkspacePrefix) {
		return ctrl.Result{}, nil
	}
	// proxy ignore clusterNamespaces ClusterResource
	if !r.isControlPlane && strings.HasPrefix(request.Namespace, managerCommon.ClusterWorkspacePrefix) {
		return ctrl.Result{}, nil
	}

	r.log.Info(fmt.Sprintf("Reconciling ClusterResource(%s:%s)", request.Namespace, request.Name))

	// get ClusterResource
	instance := &v1alpha1.ClusterResource{}
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
		return r.removeClusterResource(ctx, instance)
	}

	return r.syncClusterResource(ctx, instance)
}

func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.ClusterResource{}).
		Complete(r)
}

func Setup(mgr ctrl.Manager, controllerCommon controllerCommon.Args) error {
	reconciler := Reconciler{
		Client:         mgr.GetClient(),
		Scheme:         mgr.GetScheme(),
		log:            logf.Log.WithName("cluster_resource_controller"),
		isControlPlane: controllerCommon.IsControlPlane,
	}
	return reconciler.SetupWithManager(mgr)
}

// sync core/proxy clusterResource
func (r *Reconciler) syncClusterResource(ctx context.Context, instance *v1alpha1.ClusterResource) (ctrl.Result, error) {
	if r.isControlPlane {
		return r.syncCoreClusterResource(ctx, instance)
	}
	// TODO proxy should listen for the deletion of corresponding resources
	return r.syncProxyClusterResource(ctx, instance)
}

func (r *Reconciler) syncCoreClusterResource(ctx context.Context, instance *v1alpha1.ClusterResource) (ctrl.Result, error) {
	err := sendClusterResourceToProxy(SyncEventTypeUpdate, instance)
	if err != nil {
		r.log.Error(err, fmt.Sprintf("send ClusterResouce failed, resource(%s)", instance.Name))
		return controllerCommon.ReQueueResult(err)
	}
	return ctrl.Result{}, nil
}

func (r *Reconciler) syncProxyClusterResource(ctx context.Context, instance *v1alpha1.ClusterResource) (ctrl.Result, error) {
	var err error
	// create or complete status should sync resource(eg: resource deleted when clusterResource status is complete)
	err = r.syncResourceAndUpdateStatus(ctx, instance)
	if err != nil {
		r.log.Error(err, fmt.Sprintf("sync resource and update clusterResource(%s:%s) status failed", instance.Namespace, instance.Name))
		return controllerCommon.ReQueueResult(err)
	}
	return ctrl.Result{}, nil
}

func (r *Reconciler) removeClusterResource(ctx context.Context, instance *v1alpha1.ClusterResource) (ctrl.Result, error) {
	err := r.deleteClusterResource(ctx, instance)
	if err != nil {
		return controllerCommon.ReQueueResult(err)
	}
	// delete finalizer
	return r.removeFinalizer(ctx, instance)
}

func (r *Reconciler) deleteClusterResource(ctx context.Context, instance *v1alpha1.ClusterResource) error {
	if !r.isControlPlane {
		if instance.Status.Phase != common.Terminating {
			// delete resource
			err := deleteResource(ctx, r.Client, instance)
			if err != nil {
				r.log.Error(err, fmt.Sprintf("delete resource failed, ClsuterResource(%s)", instance.Name))
				return err
			}
		}
	} else {
		// send proxy the clusterResource delete event
		err := sendClusterResourceToProxy(SyncEventTypeDelete, instance)
		if err != nil {
			r.log.Error(err, fmt.Sprintf("send ClusterResouce failed, resource(%s)", instance.Name))
			return err
		}
	}
	return nil
}

// sync clusterResource Finalizer
func (r *Reconciler) addFinalizer(ctx context.Context, instance *v1alpha1.ClusterResource) (ctrl.Result, error) {
	err := controllerCommon.AddFinalizer(ctx, r.Client, instance)
	if err != nil {
		r.log.Error(err, fmt.Sprintf("append finalizer failed, resource(%s)", instance.Name))
		return controllerCommon.ReQueueResult(err)
	}
	return ctrl.Result{}, nil
}

func (r *Reconciler) removeFinalizer(ctx context.Context, instance *v1alpha1.ClusterResource) (ctrl.Result, error) {
	err := controllerCommon.RemoveFinalizer(ctx, r.Client, instance)
	if err != nil {
		r.log.Error(err, fmt.Sprintf("delete finalizer filed from resource(%s) failed", instance.Name))
		return controllerCommon.ReQueueResult(err)
	}
	return ctrl.Result{}, nil
}

// sync resource and clusterResource status
func (r *Reconciler) syncResourceAndUpdateStatus(ctx context.Context, instance *v1alpha1.ClusterResource) error {
	// create/update resource
	err := syncResource(ctx, r.Client, instance)
	if err != nil {
		r.log.Error(err, fmt.Sprintf("ClusterResource(%s:%s) sync resource failed", instance.Namespace, instance.Name))
		// update status, add sync error message
		updateStatusError := r.updateClusterResourceStatusWithCreateErrorMessage(ctx, err.Error(), instance)
		if updateStatusError != nil {
			r.log.Error(updateStatusError, fmt.Sprintf("update status failed, resource(%s)", instance.Name))
			return updateStatusError
		}
		return err
	}
	// update status,change phase to complete
	err = r.updateClusterResourceStatusWithPhaseComplete(ctx, instance)
	if err != nil {
		r.log.Error(err, fmt.Sprintf("update clusterResource(%s:%s) status to complete failed", instance.Namespace, instance.Name))
	}
	return err
}

// sync clusterResource status
func (r *Reconciler) updateClusterResourceStatusWithCreateErrorMessage(ctx context.Context, errorMessage string, instance *v1alpha1.ClusterResource) error {
	// update status errorInfo
	newStatus := newClusterResourceStatus(common.Creating, errorMessage, instance.Generation)
	err := updateClusterResourceStatus(ctx, r.Client, instance, newStatus)
	if err != nil {
		r.log.Error(err, fmt.Sprintf("update clusterResource(%s:%s) status failed", instance.Namespace, instance.Name))
	}
	return err
}

func (r *Reconciler) updateClusterResourceStatusWithPhaseComplete(ctx context.Context, instance *v1alpha1.ClusterResource) error {
	if instance.Status.Phase == common.Complete && instance.Status.ObservedReceiveGeneration == instance.Generation && len(instance.Status.Message) <= 0 {
		return nil
	}
	// update status complete
	newStatus := newClusterResourceStatus(common.Complete, "", instance.Generation)
	err := updateClusterResourceStatus(ctx, r.Client, instance, newStatus)
	if err != nil {
		r.log.Error(err, fmt.Sprintf("update clusterResource(%s:%s) status failed", instance.Namespace, instance.Name))
	}
	return err
}

func (r *Reconciler) updateClusterResourceStatusWithPhaseCreate(ctx context.Context, instance *v1alpha1.ClusterResource) error {
	if len(instance.Status.Phase) > 0 {
		return nil
	}
	newStatus := newClusterResourceStatus(common.Creating, "", instance.Generation)
	err := updateClusterResourceStatus(ctx, r.Client, instance, newStatus)
	if err != nil {
		r.log.Error(err, fmt.Sprintf("update clusterResource(%s:%s) status failed", instance.Namespace, instance.Name))
	}
	return err
}
