package resource_aggregate_policy

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"harmonycloud.cn/stellaris/config"
	managerCommon "harmonycloud.cn/stellaris/pkg/common"
	coreSender "harmonycloud.cn/stellaris/pkg/core/sender"
	"harmonycloud.cn/stellaris/pkg/model"

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

func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.ResourceAggregatePolicy{}).
		Complete(r)
}

func Setup(mgr ctrl.Manager, controllerCommon controllerCommon.Args) error {
	reconciler := Reconciler{
		Client:         mgr.GetClient(),
		Scheme:         mgr.GetScheme(),
		log:            logf.Log.WithName("resource_aggregate_policy_controller"),
		isControlPlane: controllerCommon.IsControlPlane,
	}
	return reconciler.SetupWithManager(mgr)
}

func (r *Reconciler) Reconcile(ctx context.Context, request ctrl.Request) (ctrl.Result, error) {
	// core focus on clusterNamespaces ClusterResource
	if r.isControlPlane && !strings.HasPrefix(request.Namespace, managerCommon.ClusterWorkspacePrefix) {
		return ctrl.Result{}, nil
	}
	// agent ignore clusterNamespaces ClusterResource
	if !r.isControlPlane && strings.HasPrefix(request.Namespace, managerCommon.ClusterWorkspacePrefix) {
		return ctrl.Result{}, nil
	}
	r.log.Info(fmt.Sprintf("Reconciling ResourceAggregatePolicy(%s:%s)", request.Namespace, request.Name))
	// get resource
	instance := &v1alpha1.ResourceAggregatePolicy{}
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

	return r.syncResourceAggregatePolicy(ctx, instance)
}

func (r *Reconciler) syncResourceAggregatePolicy(ctx context.Context, instance *v1alpha1.ResourceAggregatePolicy) (ctrl.Result, error) {
	if r.isControlPlane {
		// core send ResourceAggregatePolicy to proxy
		return r.syncResourceAggregatePolicyToProxy(model.AggregateUpdateOrCreate, instance)
	}
	// proxy aggregate target resource
}

func (r *Reconciler) aggregateTargetResource() {

}

// sendResourceAggregatePolicyToProxy core send ResourceAggregatePolicy to proxy with create/update/delete event
func (r *Reconciler) syncResourceAggregatePolicyToProxy(responseType model.ServiceResponseType, instance *v1alpha1.ResourceAggregatePolicy) (ctrl.Result, error) {
	policyResponse, err := newResourceAggregatePolicyResponse(responseType, instance)
	if err != nil {
		r.log.Error(err, fmt.Sprintf("marshal policy(%s:%s) failed", instance.GetNamespace(), instance.GetName()))
		return controllerCommon.ReQueueResult(err)
	}
	err = coreSender.SendResponseToAgent(policyResponse)
	if err != nil {
		r.log.Error(err, fmt.Sprintf("send policy(%s:%s) to proxy failed", instance.GetNamespace(), instance.GetName()))
		return controllerCommon.ReQueueResult(err)
	}
	return ctrl.Result{}, nil
}

func newResourceAggregatePolicyResponse(responseType model.ServiceResponseType, instance *v1alpha1.ResourceAggregatePolicy) (*config.Response, error) {
	clusterName := managerCommon.ClusterName(instance.GetName())
	policyData, err := json.Marshal(instance)
	if err != nil {
		return nil, err
	}
	return coreSender.NewResponse(responseType, clusterName, string(policyData))
}

// removeResourceAggregatePolicy core send delete ResourceAggregatePolicy response to proxy first，then remove finalizer
func (r *Reconciler) removeResourceAggregatePolicy(ctx context.Context, instance *v1alpha1.ResourceAggregatePolicy) (ctrl.Result, error) {
	if r.isControlPlane {
		_, err := r.syncResourceAggregatePolicyToProxy(model.AggregateDelete, instance)
		if err != nil {
			return controllerCommon.ReQueueResult(err)
		}
	}
	return r.removeFinalizer(ctx, instance)
}

func (r *Reconciler) addFinalizer(ctx context.Context, instance client.Object) (ctrl.Result, error) {
	err := controllerCommon.AddFinalizer(ctx, r.Client, instance)
	if err != nil {
		r.log.Error(err, fmt.Sprintf("append finalizer filed, resource(%s)", instance.GetName()))
		return controllerCommon.ReQueueResult(err)
	}
	return ctrl.Result{}, nil
}

func (r *Reconciler) removeFinalizer(ctx context.Context, instance client.Object) (ctrl.Result, error) {
	err := controllerCommon.RemoveFinalizer(ctx, r.Client, instance)
	if err != nil {
		r.log.Error(err, fmt.Sprintf("delete finalizer filed, resource(%s)", instance.GetName()))
		return controllerCommon.ReQueueResult(err)
	}
	return ctrl.Result{}, nil
}
