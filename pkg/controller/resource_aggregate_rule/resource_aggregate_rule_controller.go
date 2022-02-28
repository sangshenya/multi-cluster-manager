package resource_aggregate_rule

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"k8s.io/client-go/tools/record"

	coreSender "harmonycloud.cn/stellaris/pkg/core/sender"
	"harmonycloud.cn/stellaris/pkg/model"

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
	log            logr.Logger
	Scheme         *runtime.Scheme
	Recorder       record.EventRecorder
	isControlPlane bool
}

func (r *Reconciler) Reconcile(ctx context.Context, request ctrl.Request) (ctrl.Result, error) {
	r.log.V(4).Info(fmt.Sprintf("Start Reconciling MultiClusterResourceAggregateRule(%s:%s)", request.Namespace, request.Name))
	defer r.log.V(4).Info(fmt.Sprintf("End Reconciling MultiClusterResourceAggregateRule(%s:%s)", request.Namespace, request.Name))

	// get resource
	instance := &v1alpha1.MultiClusterResourceAggregateRule{}
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
		if r.isControlPlane {
			// TODO send request to proxy delete event
		}
		if err = controllerCommon.RemoveFinalizer(ctx, r.Client, instance); err != nil {
			r.log.Error(err, fmt.Sprintf("delete finalizer failed, resource(%s:%s)", instance.Namespace, instance.Name))
			r.Recorder.Event(instance, "Warning", "FailedDeleteFinalizers", err.Error())
			return ctrl.Result{Requeue: true, RequeueAfter: 5 * time.Second}, err
		}
		return ctrl.Result{}, nil
	}

	if err = r.syncResourceAggregateRule(ctx, instance); err != nil {
		r.log.Error(err, fmt.Sprintf("sync rule failed, resource(%s:%s)", instance.Namespace, instance.Name))
		r.Recorder.Event(instance, "Warning", "FailedSyncRule", err.Error())
		return ctrl.Result{Requeue: true, RequeueAfter: 5 * time.Second}, err
	}
	return ctrl.Result{}, nil
}

func (r *Reconciler) syncResourceAggregateRule(ctx context.Context, instance *v1alpha1.MultiClusterResourceAggregateRule) error {
	if shouldAddRuleLabels(instance) {
		return r.addRuleLabels(ctx, instance)
	}
	if r.isControlPlane {
		return nil
	}
	// send rule to proxy
	return r.sendAggregateRuleToProxy(ctx, instance)
}

func (r *Reconciler) sendAggregateRuleToProxy(ctx context.Context, instance *v1alpha1.MultiClusterResourceAggregateRule) error {
	aggregateModel := &model.SyncAggregateResourceModel{
		RuleList: []v1alpha1.MultiClusterResourceAggregateRule{*instance},
	}
	jsonString, err := json.Marshal(aggregateModel)
	if err != nil {
		err = fmt.Errorf(fmt.Sprintf("marshal aggregate model failed, rule(%s:%s)", instance.Namespace, instance.Name), err)
		return err
	}
	// get all cluster
	clusterList, err := controllerCommon.AllCluster(ctx, r.Client)
	if err != nil {
		err = fmt.Errorf(fmt.Sprintf("get all cluster failed, rule(%s:%s)", instance.Namespace, instance.Name), err)
		return err
	}
	for _, cluster := range clusterList.Items {
		if len(cluster.GetName()) <= 0 || cluster.Status.Status == v1alpha1.OfflineStatus {
			r.log.Info(fmt.Sprintf("clusterName is empty or cluster status is offline"))
			continue
		}
		ruleResponse, err := coreSender.NewResponse(model.AggregateUpdateOrCreate, cluster.GetName(), string(jsonString))
		if err != nil {
			err = fmt.Errorf(fmt.Sprintf("new rule response failed, rule(%s:%s)", instance.Namespace, instance.Name), err)
			return err
		}
		err = coreSender.SendResponseToProxy(ruleResponse)
		if err != nil {
			err = fmt.Errorf(fmt.Sprintf("send rule response failed, rule(%s:%s)", instance.Namespace, instance.Name), err)
			return err
		}
	}
	return nil
}

func (r *Reconciler) addRuleLabels(ctx context.Context, instance *v1alpha1.MultiClusterResourceAggregateRule) error {
	ruleLabels := instance.GetLabels()
	targetGvkString := managerCommon.GvkLabelString(instance.Spec.ResourceRef)
	ruleLabels[managerCommon.AggregateResourceGvkLabelName] = targetGvkString
	instance.SetLabels(ruleLabels)
	err := r.Client.Update(ctx, instance)
	if err != nil {
		return err
	}
	return nil
}

func shouldAddRuleLabels(instance *v1alpha1.MultiClusterResourceAggregateRule) bool {
	gvkString, ok := instance.GetLabels()[managerCommon.AggregateResourceGvkLabelName]
	targetGvkString := managerCommon.GvkLabelString(instance.Spec.ResourceRef)
	if ok && gvkString == targetGvkString {
		return false
	}
	return true
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
	if controllerCommon.IsControlPlane {
		reconciler.Recorder = mgr.GetEventRecorderFor("stellaris-core")
	} else {
		reconciler.Recorder = mgr.GetEventRecorderFor("stellaris-proxy")
	}
	return reconciler.SetupWithManager(mgr)
}
