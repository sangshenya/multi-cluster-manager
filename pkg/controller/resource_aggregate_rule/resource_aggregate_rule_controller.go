package resource_aggregate_rule

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"harmonycloud.cn/stellaris/pkg/common/helper"

	"k8s.io/apimachinery/pkg/labels"

	"k8s.io/client-go/tools/record"

	coreSender "harmonycloud.cn/stellaris/pkg/core/sender"
	"harmonycloud.cn/stellaris/pkg/model"
	sliceutil "harmonycloud.cn/stellaris/pkg/utils/slice"

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
			if err = r.updateMultiResourceAggregatePolicy(ctx, instance); err != nil {
				r.log.Error(err, fmt.Sprintf("update multiPolicy failed, resource(%s:%s)", instance.Namespace, instance.Name))
				r.Recorder.Event(instance, "Warning", "FailedUpdateMultiPolicy", err.Error())
				return ctrl.Result{Requeue: true, RequeueAfter: 5 * time.Second}, err
			}
			if err = r.sendAggregateRuleToProxy(ctx, model.AggregateDelete, instance); err != nil {
				r.log.Error(err, fmt.Sprintf("send delete event to proxy failed, resource(%s:%s)", instance.Namespace, instance.Name))
				r.Recorder.Event(instance, "Warning", "FailedSendDeleteEventToProxy", err.Error())
				return ctrl.Result{Requeue: true, RequeueAfter: 5 * time.Second}, err
			}
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

func (r *Reconciler) syncResourceAggregateRule(
	ctx context.Context,
	instance *v1alpha1.MultiClusterResourceAggregateRule) error {
	if shouldAddRuleLabels(instance) {
		return r.addRuleLabels(ctx, instance)
	}
	if r.isControlPlane {
		return nil
	}
	// send rule to proxy
	return r.sendAggregateRuleToProxy(ctx, model.AggregateUpdateOrCreate, instance)
}

func (r *Reconciler) updateMultiResourceAggregatePolicy(ctx context.Context, instance *v1alpha1.MultiClusterResourceAggregateRule) error {
	multiPolicyList, err := r.getMultiPolicyList(ctx, instance)
	if err != nil {
		return err
	}
	if len(multiPolicyList.Items) == 0 {
		return nil
	}
	for _, multiPolicy := range multiPolicyList.Items {
		if !sliceutil.ContainsString(multiPolicy.Spec.AggregateRules, instance.GetName()) {
			continue
		}
		if len(multiPolicy.Spec.AggregateRules) == 1 {
			// delete multiPolicy
			if err = r.Client.Delete(ctx, &multiPolicy); err != nil {
				return err
			}
			continue
		}
		multiPolicy.Spec.AggregateRules = sliceutil.RemoveString(multiPolicy.Spec.AggregateRules, instance.GetName())
		if err = r.Client.Update(ctx, &multiPolicy); err != nil {
			return err
		}
	}
	return nil
}

// sendAggregateRuleToProxy send rule to proxy when rule update/create/delete
func (r *Reconciler) sendAggregateRuleToProxy(
	ctx context.Context,
	eventType model.ServiceResponseType,
	instance *v1alpha1.MultiClusterResourceAggregateRule) error {
	aggregateModel := &model.SyncAggregateResourceModel{
		RuleList: []v1alpha1.MultiClusterResourceAggregateRule{*instance},
	}
	jsonString, err := json.Marshal(aggregateModel)
	if err != nil {
		return errors.New("marshal aggregate model failed" + err.Error())
	}
	// get all cluster
	clusterList, err := controllerCommon.AllCluster(ctx, r.Client)
	if err != nil {
		return errors.New("get all cluster failed" + err.Error())
	}
	for _, cluster := range clusterList.Items {
		if len(cluster.GetName()) == 0 || cluster.Status.Status == v1alpha1.OfflineStatus {
			r.log.Info("clusterName is empty or cluster status is offline")
			continue
		}
		mappingNs, err := helper.GetMappingNamespace(ctx, r.Client, cluster.Name, instance.Namespace)
		if err != nil {
			return errors.New("get mapping namespace failed," + err.Error())
		}
		if mappingNs != instance.Namespace {
			instance.Namespace = mappingNs
			aggregateModel.RuleList[0] = *instance
		}
		ruleResponse, err := coreSender.NewResponse(eventType, cluster.GetName(), string(jsonString))
		if err != nil {
			return errors.New("new rule response failed," + err.Error())
		}
		err = coreSender.SendResponseToProxy(ruleResponse)
		if err != nil {
			return errors.New("send rule response failed," + err.Error())
		}
	}
	return nil
}

func (r *Reconciler) addRuleLabels(ctx context.Context, instance *v1alpha1.MultiClusterResourceAggregateRule) error {
	ruleLabels := instance.GetLabels()
	if ruleLabels == nil {
		ruleLabels = make(map[string]string)
	}
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

func (r *Reconciler) getMultiPolicyList(
	ctx context.Context,
	instance *v1alpha1.MultiClusterResourceAggregateRule) (*v1alpha1.MultiClusterResourceAggregatePolicyList, error) {
	selector, err := labels.Parse(managerCommon.AggregateRuleLabelName + "." + instance.GetName() + "=1")
	if err != nil {
		return nil, err
	}
	var multiPolicyList *v1alpha1.MultiClusterResourceAggregatePolicyList
	err = r.Client.List(ctx, multiPolicyList, &client.ListOptions{
		LabelSelector: selector,
	})
	return multiPolicyList, err
}
