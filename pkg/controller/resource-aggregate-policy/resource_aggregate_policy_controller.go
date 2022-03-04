package resource_aggregate_policy

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"k8s.io/client-go/tools/record"

	"harmonycloud.cn/stellaris/config"
	managerCommon "harmonycloud.cn/stellaris/pkg/common"
	coreSender "harmonycloud.cn/stellaris/pkg/core/sender"
	"harmonycloud.cn/stellaris/pkg/model"

	"github.com/go-logr/logr"
	"harmonycloud.cn/stellaris/pkg/apis/multicluster/v1alpha1"
	controllerCommon "harmonycloud.cn/stellaris/pkg/controller/common"
	proxyAggregate "harmonycloud.cn/stellaris/pkg/proxy/aggregate"
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
	Recorder       record.EventRecorder
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
	if controllerCommon.IsControlPlane {
		reconciler.Recorder = mgr.GetEventRecorderFor("stellaris-core")
	} else {
		reconciler.Recorder = mgr.GetEventRecorderFor("stellaris-proxy")
	}
	return reconciler.SetupWithManager(mgr)
}

func (r *Reconciler) Reconcile(ctx context.Context, request ctrl.Request) (ctrl.Result, error) {
	// core focus on clusterNamespaces ResourceAggregatePolicy
	if r.isControlPlane && !strings.HasPrefix(request.Namespace, managerCommon.ClusterNamespaceInControlPlanePrefix) {
		return ctrl.Result{}, nil
	}
	// proxy ignore clusterNamespaces ResourceAggregatePolicy
	if !r.isControlPlane && strings.HasPrefix(request.Namespace, managerCommon.ClusterNamespaceInControlPlanePrefix) {
		return ctrl.Result{}, nil
	}

	r.log.V(4).Info(fmt.Sprintf("Start Reconciling ResourceAggregatePolicy(%s:%s)", request.Namespace, request.Name))
	defer r.log.V(4).Info(fmt.Sprintf("End Reconciling ResourceAggregatePolicy(%s:%s)", request.Namespace, request.Name))

	// get resource
	instance := &v1alpha1.ResourceAggregatePolicy{}
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
		if err = r.deleteResourceAggregatePolicy(ctx, instance); err != nil {
			r.log.Error(err, fmt.Sprintf("delete finalizer plan failed, resource(%s:%s)", instance.Namespace, instance.Name))
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

	if err = r.syncResourceAggregatePolicy(ctx, instance); err != nil {
		r.log.Error(err, fmt.Sprintf("sync ResourceAggregatePolicy failed, resource(%s:%s)", instance.Namespace, instance.Name))
		return ctrl.Result{Requeue: true, RequeueAfter: 5 * time.Second}, err
	}
	return ctrl.Result{}, nil
}

// syncResourceAggregatePolicy
// core send ResourceAggregatePolicy create or update event to proxy
// proxy aggregate target resource, add config, add informer
func (r *Reconciler) syncResourceAggregatePolicy(ctx context.Context, instance *v1alpha1.ResourceAggregatePolicy) error {
	if r.isControlPlane {
		// core send ResourceAggregatePolicy to proxy
		return r.syncResourceAggregatePolicyToProxy(model.AggregateUpdateOrCreate, instance)
	}
	// proxy aggregate target resource, add config, add informer
	return proxyAggregate.AddResourceAggregatePolicy(instance)
}

// deleteResourceAggregatePolicy
// core send ResourceAggregatePolicy delete event to proxy
// proxy delete config and informer
func (r *Reconciler) deleteResourceAggregatePolicy(ctx context.Context, instance *v1alpha1.ResourceAggregatePolicy) error {
	if r.isControlPlane {
		return r.syncResourceAggregatePolicyToProxy(model.AggregateDelete, instance)
	}
	return proxyAggregate.RemoveResourceAggregatePolicy(instance)
}

// sendResourceAggregatePolicyToProxy core send ResourceAggregatePolicy to proxy with create/update/delete event
func (r *Reconciler) syncResourceAggregatePolicyToProxy(responseType model.ServiceResponseType, instance *v1alpha1.ResourceAggregatePolicy) error {
	policyResponse, err := newResourceAggregatePolicyResponse(responseType, instance)
	if err != nil {
		err = fmt.Errorf(fmt.Sprintf("marshal policy(%s:%s) failed", instance.GetNamespace(), instance.GetName()), err)
		return err
	}
	err = coreSender.SendResponseToProxy(policyResponse)
	if err != nil {
		err = fmt.Errorf(fmt.Sprintf("send policy(%s:%s) to proxy failed", instance.GetNamespace(), instance.GetName()), err)
		return err
	}
	return nil
}

func newResourceAggregatePolicyResponse(responseType model.ServiceResponseType, instance *v1alpha1.ResourceAggregatePolicy) (*config.Response, error) {
	clusterName := managerCommon.ClusterName(instance.GetName())
	policyData, err := json.Marshal(instance)
	if err != nil {
		return nil, err
	}
	return coreSender.NewResponse(responseType, clusterName, string(policyData))
}
