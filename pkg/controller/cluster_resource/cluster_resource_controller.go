package cluster_resource

import (
	"context"
	"fmt"
	"time"

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
	instance := &v1alpha1.MultiClusterResourceBinding{}
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
		instance.ObjectMeta.Finalizers = sliceutil.RemoveString(instance.ObjectMeta.Finalizers, managerCommon.FinalizerName)
		if err = r.Client.Update(ctx, instance); err != nil {
			return ctrl.Result{
				Requeue:      true,
				RequeueAfter: 30 * time.Second,
			}, fmt.Errorf("delete finalizer filed from resource %s failed: %s", instance.Name, err)
		}
		return ctrl.Result{}, nil
	}

	// TODO send agent clusterResource

	// TODO sync binding status

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
