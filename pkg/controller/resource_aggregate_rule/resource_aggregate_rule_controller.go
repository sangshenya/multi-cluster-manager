package resource_aggregate_rule

import (
	"context"
	"fmt"

	managerCommon "harmonycloud.cn/stellaris/pkg/common"

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
	log    logr.Logger
	Scheme *runtime.Scheme
}

func (r *Reconciler) Reconcile(ctx context.Context, request ctrl.Request) (ctrl.Result, error) {
	r.log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	r.log.Info("Reconciling MultiClusterResourceAggregateRule")
	// get resource
	instance := &v1alpha1.MultiClusterResourceAggregateRule{}
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
		return r.removeFinalizer(ctx, instance)
	}

	return r.syncResourceAggregateRule(ctx, instance)
}

func (r *Reconciler) syncResourceAggregateRule(ctx context.Context, instance *v1alpha1.MultiClusterResourceAggregateRule) (ctrl.Result, error) {
	ruleLabels := instance.GetLabels()
	gvkString, ok := ruleLabels[managerCommon.AggregateResourceGvkLabelName]
	targetGvkString := managerCommon.GvkLabelString(instance.Spec.ResourceRef)
	if ok && gvkString == targetGvkString {
		return ctrl.Result{}, nil
	}
	ruleLabels[managerCommon.AggregateResourceGvkLabelName] = targetGvkString
	instance.SetLabels(ruleLabels)
	err := r.Client.Update(ctx, instance)
	if err != nil {
		return controllerCommon.ReQueueResult(err)
	}
	return ctrl.Result{}, nil
}

func (r *Reconciler) addFinalizer(ctx context.Context, instance *v1alpha1.MultiClusterResourceAggregateRule) (ctrl.Result, error) {
	err := controllerCommon.AddFinalizer(ctx, r.Client, instance)
	if err != nil {
		r.log.Error(err, fmt.Sprintf("append finalizer filed, resource(%s)", instance.Name))
		return controllerCommon.ReQueueResult(err)
	}
	return ctrl.Result{}, nil
}

func (r *Reconciler) removeFinalizer(ctx context.Context, instance *v1alpha1.MultiClusterResourceAggregateRule) (ctrl.Result, error) {
	err := controllerCommon.RemoveFinalizer(ctx, r.Client, instance)
	if err != nil {
		r.log.Error(err, fmt.Sprintf("delete finalizer filed, resource(%s)", instance.Name))
		return controllerCommon.ReQueueResult(err)
	}
	return ctrl.Result{}, nil
}

func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.MultiClusterResourceAggregateRule{}).
		Complete(r)
}

func Setup(mgr ctrl.Manager, controllerCommon controllerCommon.Args) error {
	reconciler := Reconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
		log:    logf.Log.WithName("resource_aggregate_rule_controller"),
	}
	return reconciler.SetupWithManager(mgr)
}
