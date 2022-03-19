package cluster_resource

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"k8s.io/client-go/tools/record"

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
	Recorder       record.EventRecorder
	isControlPlane bool
}

func (r *Reconciler) Reconcile(ctx context.Context, request ctrl.Request) (ctrl.Result, error) {
	// core focus on clusterNamespaces ClusterResource
	if r.isControlPlane && !strings.HasPrefix(request.Namespace, managerCommon.ClusterNamespaceInControlPlanePrefix) {
		return ctrl.Result{}, nil
	}
	// proxy ignore clusterNamespaces ClusterResource
	if !r.isControlPlane && strings.HasPrefix(request.Namespace, managerCommon.ClusterNamespaceInControlPlanePrefix) {
		return ctrl.Result{}, nil
	}

	r.log.V(4).Info(fmt.Sprintf("Start Reconciling ClusterResource(%s:%s)", request.Namespace, request.Name))
	defer r.log.V(4).Info(fmt.Sprintf("End Reconciling ClusterResource(%s:%s)", request.Namespace, request.Name))

	// get ClusterResource
	instance := &v1alpha1.ClusterResource{}
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
		if err = r.deleteClusterResourcePlan(ctx, instance); err != nil {
			r.log.Error(err, fmt.Sprintf("delete finalizer failed, resource(%s:%s)", instance.Namespace, instance.Name))
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

	err = r.syncClusterResource(ctx, instance)
	if err != nil {
		r.log.Error(err, fmt.Sprintf("sync clusterResource failed, resource(%s:%s)", instance.Namespace, instance.Name))
		r.Recorder.Event(instance, "Warning", "FailedSyncClusterResource", err.Error())
		return ctrl.Result{Requeue: true, RequeueAfter: 5 * time.Second}, err
	}
	return ctrl.Result{}, nil
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
	if controllerCommon.IsControlPlane {
		reconciler.Recorder = mgr.GetEventRecorderFor("stellaris-core")
	} else {
		reconciler.Recorder = mgr.GetEventRecorderFor("stellaris-proxy")
	}
	return reconciler.SetupWithManager(mgr)
}

// sync core/proxy clusterResource
func (r *Reconciler) syncClusterResource(ctx context.Context, instance *v1alpha1.ClusterResource) error {
	if r.isControlPlane {
		return sendClusterResourceToProxy(SyncEventTypeUpdate, instance)
	}
	// TODO proxy should listen for the deletion of corresponding resources
	return r.syncResourceAndUpdateStatus(ctx, instance)
}

func (r *Reconciler) deleteClusterResourcePlan(ctx context.Context, instance *v1alpha1.ClusterResource) error {
	if !r.isControlPlane {
		// delete resource
		return deleteResource(ctx, r.Client, instance)
	}
	return sendClusterResourceToProxy(SyncEventTypeDelete, instance)
}

// sync resource and clusterResource status
func (r *Reconciler) syncResourceAndUpdateStatus(ctx context.Context, instance *v1alpha1.ClusterResource) error {
	// create/update resource
	err := syncResource(ctx, r.Client, instance)
	if err != nil {
		err = errors.New("ClusterResource sync resource failed," + err.Error())
		// update status, add sync error message
		updateStatusError := r.updateClusterResourceStatusWithCreateErrorMessage(ctx, err.Error(), instance)
		if updateStatusError != nil {
			updateStatusError = errors.New("update status failed," + updateStatusError.Error())
			return updateStatusError
		}
		return err
	}
	// update status,change phase to complete
	err = r.updateClusterResourceStatusWithPhaseComplete(ctx, instance)
	if err != nil {
		err = errors.New("update clusterResource failed when update status to complete," + err.Error())
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
	if instance.Status.Phase == common.Complete && instance.Status.ObservedReceiveGeneration == instance.Generation && len(instance.Status.Message) == 0 {
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
