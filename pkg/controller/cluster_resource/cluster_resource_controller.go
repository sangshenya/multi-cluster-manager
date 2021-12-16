package cluster_resource

import (
	"context"
	"fmt"
	"time"

	"harmonycloud.cn/stellaris/pkg/apis/multicluster/common"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"

	"github.com/go-logr/logr"
	"harmonycloud.cn/stellaris/pkg/apis/multicluster/v1alpha1"
	managerCommon "harmonycloud.cn/stellaris/pkg/common"
	controllerCommon "harmonycloud.cn/stellaris/pkg/controller/common"
	sliceutil "harmonycloud.cn/stellaris/pkg/util/slice"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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
	r.log.Info("Reconciling MultiClusterResourceBinding")

	// get resource
	instance := &v1alpha1.ClusterResource{}
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
			return ctrl.Result{
				Requeue:      true,
				RequeueAfter: 30 * time.Second,
			}, fmt.Errorf("append finalizer filed to resource %s failed: %s", instance.Name, err)
		}
		return ctrl.Result{}, nil
	}

	// the object is being deleted
	if !instance.ObjectMeta.DeletionTimestamp.IsZero() && sliceutil.ContainsString(instance.ObjectMeta.Finalizers, managerCommon.FinalizerName) {
		if !managerCommon.IsControlPlane() {
			if instance.Status.Phase != common.Terminating {
				// update status
				instance.Status.Message = ""
				instance.Status.ObservedReceiveGeneration = instance.Generation
				instance.Status.Phase = common.Terminating

				err = updateClusterResourceStatus(ctx, r.Client, instance)
				if err != nil {
					return ctrl.Result{
						Requeue:      true,
						RequeueAfter: 30 * time.Second,
					}, fmt.Errorf("update status failed:%s, resource(%s)", err, instance.Name)
				}
				return ctrl.Result{}, nil
			}
		} else {
			//
			err = sendClusterResourceToAgent(SyncEventTypeDelete, instance)
			if err != nil {
				return ctrl.Result{
					Requeue:      true,
					RequeueAfter: 30 * time.Second,
				}, fmt.Errorf("send ClusterResouce failed:%s, resource(%s)", err, instance.Name)
			}
		}

		instance.ObjectMeta.Finalizers = sliceutil.RemoveString(instance.ObjectMeta.Finalizers, managerCommon.FinalizerName)
		if err = r.Client.Update(ctx, instance); err != nil {
			return ctrl.Result{
				Requeue:      true,
				RequeueAfter: 30 * time.Second,
			}, fmt.Errorf("delete finalizer filed from resource %s failed: %s", instance.Name, err)
		}

		return ctrl.Result{}, nil
	}

	if managerCommon.IsControlPlane() {
		// TODO send clusterResource to agent
		err = sendClusterResourceToAgent(SyncEventTypeUpdate, instance)
		if err != nil {
			return ctrl.Result{
				Requeue:      true,
				RequeueAfter: 30 * time.Second,
			}, fmt.Errorf("send ClusterResouce failed:%s, resource(%s)", err, instance.Name)
		}
		return ctrl.Result{}, nil
	} else {
		if instance.Generation != instance.Status.ObservedReceiveGeneration {
			// update status to creating
			instance.Status.ObservedReceiveGeneration = instance.Generation
			instance.Status.Phase = common.Creating
			instance.Status.Message = ""

			err = updateClusterResourceStatus(ctx, r.Client, instance)
			if err != nil {
				return ctrl.Result{
					Requeue:      true,
					RequeueAfter: 30 * time.Second,
				}, fmt.Errorf("update status failed:%s, resource(%s)", err, instance.Name)
			}
			return ctrl.Result{}, nil
		}
		if instance.Status.Phase == common.Creating {
			// create/update resource
			err = syncResource(ctx, r.Client, instance.Spec.Resource)
			if err != nil {
				instance.Status.Message = err.Error()

				err = updateClusterResourceStatus(ctx, r.Client, instance)
				if err != nil {
					return ctrl.Result{
						Requeue:      true,
						RequeueAfter: 30 * time.Second,
					}, fmt.Errorf("update status failed:%s, resource(%s)", err, instance.Name)
				}

				return ctrl.Result{
					RequeueAfter: 30 * time.Second,
					Requeue:      true,
				}, fmt.Errorf("sync resource fail, resource %s failed: %s", instance.Name, instance.Status.Message)
			}
			instance.Status.Message = ""
			instance.Status.Phase = common.Complete
			instance.Status.ObservedReceiveGeneration = instance.Generation

			err = updateClusterResourceStatus(ctx, r.Client, instance)
			if err != nil {
				return ctrl.Result{
					Requeue:      true,
					RequeueAfter: 30 * time.Second,
				}, fmt.Errorf("update status failed:%s, resource(%s)", err, instance.Name)
			}
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

func syncResource(ctx context.Context, clientSet client.Client, resource *runtime.RawExtension) error {
	resourceObject, err := formatDataToUnstructured(resource.Raw)
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

func formatDataToUnstructured(data []byte) (*unstructured.Unstructured, error) {
	// Decode YAML manifest into unstructured.Unstructured
	obj := &unstructured.Unstructured{}
	dec := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
	_, _, err := dec.Decode(data, nil, obj)
	if err != nil {
		return nil, err
	}
	return obj, nil
}

func updateClusterResourceStatus(ctx context.Context, clientSet client.Client, clusterResource *v1alpha1.ClusterResource) error {
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
