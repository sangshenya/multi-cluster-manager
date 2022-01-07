package cluster_resource

import (
	"context"
	"errors"
	"fmt"

	"harmonycloud.cn/stellaris/pkg/apis/multicluster/common"

	"github.com/go-logr/logr"
	"harmonycloud.cn/stellaris/pkg/apis/multicluster/v1alpha1"
	managerCommon "harmonycloud.cn/stellaris/pkg/common"
	controllerCommon "harmonycloud.cn/stellaris/pkg/controller/common"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
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
	r.log.Info("Reconciling ClusterResource")

	// get ClusterResource
	instance := &v1alpha1.ClusterResource{}
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
			r.log.Error(err, fmt.Sprintf("append finalizer filed to resource(%s) failed", instance.Name))
			return controllerCommon.ReQueueResult(err)
		}
		return ctrl.Result{}, nil
	}

	// the object is being deleted
	if !instance.GetDeletionTimestamp().IsZero() {
		err = r.deleteClusterResource(ctx, instance)
		if err != nil {
			return controllerCommon.ReQueueResult(err)
		}
		// delete finalizer
		err = controllerCommon.RemoveFinalizer(ctx, r.Client, instance)
		if err != nil {
			r.log.Error(err, fmt.Sprintf("delete finalizer filed from resource(%s) failed", instance.Name))
			return controllerCommon.ReQueueResult(err)
		}
		return ctrl.Result{}, nil
	}
	// core
	if managerCommon.IsControlPlane() {
		err = sendClusterResourceToAgent(SyncEventTypeUpdate, instance)
		if err != nil {
			r.log.Error(err, fmt.Sprintf("send ClusterResouce failed, resource(%s)", instance.Name))
			return controllerCommon.ReQueueResult(err)
		}
		return ctrl.Result{}, nil
	}
	// agent
	if len(instance.Status.Phase) == 0 || instance.Generation != instance.Status.ObservedReceiveGeneration {
		// update status to creating
		err = r.updateClusterResourceStatusWithPhaseCreate(ctx, instance)
		if err != nil {
			r.log.Error(err, fmt.Sprintf("update status failed, resource(%s)", instance.Name))
			return controllerCommon.ReQueueResult(err)
		}
		return ctrl.Result{}, nil
	}
	if instance.Status.Phase == common.Creating {
		err = r.syncResourceAndUpdateStatus(ctx, instance)
		if err != nil {
			return controllerCommon.ReQueueResult(err)
		}
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
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
		log:    logf.Log.WithName("cluster_resource_controller"),
	}
	return reconciler.SetupWithManager(mgr)
}

func (r *Reconciler) syncResourceAndUpdateStatus(ctx context.Context, instance *v1alpha1.ClusterResource) error {
	// create/update resource
	err := syncResource(ctx, r.Client, instance.Spec.Resource)
	if err != nil {
		// update status, add sync error message
		updateStatusError := r.updateClusterResourceStatusWithCreateErrorMessage(ctx, err.Error(), instance)
		if updateStatusError != nil {
			r.log.Error(updateStatusError, fmt.Sprintf("update status failed, resource(%s)", instance.Name))
			return updateStatusError
		}
		r.log.Error(err, fmt.Sprintf("sync resource fail, resource (%s)", instance.Name))
		return err
	}
	// update status,change phase to complete
	err = r.updateClusterResourceStatusWithPhaseComplete(ctx, instance)
	r.log.Error(err, fmt.Sprintf("update status failed, resource(%s)", instance.Name))
	return err
}

func (r *Reconciler) updateClusterResourceStatusWithCreateErrorMessage(ctx context.Context, errorMessage string, instance *v1alpha1.ClusterResource) error {
	// update status errorInfo
	newStatus := newClusterResourceStatus(instance.Status.Phase, errorMessage, instance.Status.ObservedReceiveGeneration)
	return updateClusterResourceStatus(ctx, r.Client, instance, newStatus)
}

func (r *Reconciler) updateClusterResourceStatusWithPhaseComplete(ctx context.Context, instance *v1alpha1.ClusterResource) error {
	// update status complete
	newStatus := newClusterResourceStatus(common.Complete, "", instance.Generation)
	return updateClusterResourceStatus(ctx, r.Client, instance, newStatus)
}

func (r *Reconciler) updateClusterResourceStatusWithPhaseCreate(ctx context.Context, instance *v1alpha1.ClusterResource) error {
	newStatus := newClusterResourceStatus(common.Creating, "", instance.Generation)
	return updateClusterResourceStatus(ctx, r.Client, instance, newStatus)
}

// syncResource create or update resource
func syncResource(ctx context.Context, clientSet client.Client, resource *runtime.RawExtension) error {
	resourceObject, err := getObjectForRawExtension(resource)
	if err != nil {
		return err
	}
	err = clientSet.Create(ctx, resourceObject)
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return err
		}
		err = clientSet.Update(ctx, resourceObject)
		if err != nil {
			return err
		}
	}
	return nil
}

func deleteResource(ctx context.Context, clientSet client.Client, resource *runtime.RawExtension) error {
	resourceObject, err := getObjectForRawExtension(resource)
	if err != nil {
		return err
	}

	err = clientSet.Delete(ctx, resourceObject)
	if apierrors.IsNotFound(err) {
		return nil
	}
	return err
}

func getObjectForRawExtension(resource *runtime.RawExtension) (*unstructured.Unstructured, error) {
	if resource == nil {
		return nil, errors.New("resource is empty")
	}
	resourceByte, err := resource.MarshalJSON()
	if err != nil {
		return nil, err
	}
	resourceObject := &unstructured.Unstructured{}
	err = resourceObject.UnmarshalJSON(resourceByte)
	return resourceObject, err
}

func newClusterResourceStatus(phase common.MultiClusterResourcePhase, message string, generation int64) v1alpha1.ClusterResourceStatus {
	return v1alpha1.ClusterResourceStatus{
		ObservedReceiveGeneration: generation,
		Phase:                     phase,
		Message:                   message,
	}
}

// updateClusterResourceStatus send update status request to control plane, then update clusterResource status
func updateClusterResourceStatus(ctx context.Context, clientSet client.Client, clusterResource *v1alpha1.ClusterResource, status v1alpha1.ClusterResourceStatus) error {
	clusterResource.Status = status
	err := sendStatusToControlPlane(&clusterResource.Status)
	if err != nil {
		return err
	}
	return clientSet.Status().Update(ctx, clusterResource)
}

func sendStatusToControlPlane(resourceStatus *v1alpha1.ClusterResourceStatus) error {
	// TODO send status to controlPlane
	return nil
}

type SyncEventType string

const (
	SyncEventTypeUpdate SyncEventType = "update"
	SyncEventTypeDelete SyncEventType = "delete"
)

func sendClusterResourceToAgent(eventType SyncEventType, clusterResource *v1alpha1.ClusterResource) error {
	// TODO send clusterResource to agent
	return nil
}

func (r *Reconciler) deleteClusterResource(ctx context.Context, instance *v1alpha1.ClusterResource) error {
	if !managerCommon.IsControlPlane() {
		if instance.Status.Phase != common.Terminating {
			// delete resource
			err := deleteResource(ctx, r.Client, instance.Spec.Resource)
			if err != nil {
				r.log.Error(err, fmt.Sprintf("delete resource failed, ClsuterResource(%s)", instance.Name))
				return err
			}
		}
	} else {
		// send agent the clusterResource delete event
		err := sendClusterResourceToAgent(SyncEventTypeDelete, instance)
		if err != nil {
			r.log.Error(err, fmt.Sprintf("send ClusterResouce failed, resource(%s)", instance.Name))
			return err
		}
	}
	return nil
}
