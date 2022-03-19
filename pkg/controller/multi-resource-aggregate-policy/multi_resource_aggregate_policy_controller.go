package multi_resource_aggregate_policy

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"k8s.io/client-go/tools/record"

	"harmonycloud.cn/stellaris/pkg/utils/common"

	"github.com/go-logr/logr"
	"harmonycloud.cn/stellaris/pkg/apis/multicluster/v1alpha1"
	managerCommon "harmonycloud.cn/stellaris/pkg/common"
	controllerCommon "harmonycloud.cn/stellaris/pkg/controller/common"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

type Reconciler struct {
	client.Client
	log      logr.Logger
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

func (r *Reconciler) Reconcile(ctx context.Context, request ctrl.Request) (ctrl.Result, error) {
	r.log.V(4).Info(fmt.Sprintf("Start Reconciling MultiClusterResourceAggregatePolicy(%s:%s)", request.Namespace, request.Name))
	defer r.log.V(4).Info(fmt.Sprintf("End Reconciling MultiClusterResourceAggregatePolicy(%s:%s)", request.Namespace, request.Name))

	// get resource
	instance := &v1alpha1.MultiClusterResourceAggregatePolicy{}
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
			r.log.Error(err, fmt.Sprintf("delete ResourceAggregatePolicy failed, resource(%s:%s)", instance.Namespace, instance.Name))
			r.Recorder.Event(instance, "Warning", "FailedDeleteResourceAggregatePolicy", err.Error())
			return ctrl.Result{Requeue: true, RequeueAfter: 5 * time.Second}, err
		}
		if err = controllerCommon.RemoveFinalizer(ctx, r.Client, instance); err != nil {
			r.log.Error(err, fmt.Sprintf("delete finalizer filed, resource(%s:%s)", instance.Namespace, instance.Name))
			r.Recorder.Event(instance, "Warning", "FailedDeleteFinalizers", err.Error())
			return ctrl.Result{Requeue: true, RequeueAfter: 5 * time.Second}, err
		}
		return ctrl.Result{}, nil
	}

	// add labels
	if shouldChangePolicyLabels(instance) {
		if err = r.addPolicyLabels(ctx, instance); err != nil {
			r.log.Error(err, fmt.Sprintf("add labels filed, resource(%s:%s)", instance.Namespace, instance.Name))
			return ctrl.Result{Requeue: true, RequeueAfter: 5 * time.Second}, err
		}
		return ctrl.Result{}, nil
	}

	if err = r.syncResourceAggregatePolicy(ctx, instance); err != nil {
		r.log.Error(err,
			fmt.Sprintf("sync ResourceAggregatePolicy filed, resource(%s:%s)",
				instance.Namespace,
				instance.Name))
		return ctrl.Result{Requeue: true, RequeueAfter: 5 * time.Second}, err
	}
	return ctrl.Result{}, nil
}

func (r *Reconciler) deleteResourceAggregatePolicy(ctx context.Context, instance *v1alpha1.MultiClusterResourceAggregatePolicy) error {
	policyList, err := getResourceAggregatePolicyList(ctx, r.Client, common.NewNamespacedName(instance.GetNamespace(), instance.GetName()))
	if err != nil {
		return err
	}
	for _, item := range policyList.Items {
		err = r.Client.Delete(ctx, &item)
		if err != nil {
			return err
		}
	}
	return nil
}

// syncResourceAggregatePolicy update or create or delete ResourceAggregatePolicy when mPolicy update or create
func (r *Reconciler) syncResourceAggregatePolicy(
	ctx context.Context,
	instance *v1alpha1.MultiClusterResourceAggregatePolicy) error {
	if len(instance.Spec.AggregateRules) == 0 || instance.Spec.Clusters == nil {
		return nil
	}
	clusterNamespaces, err := controllerCommon.GetClusterNamespaces(
		ctx,
		r.Client,
		instance.Spec.Clusters.ClusterType,
		instance.Spec.Clusters.Clusters,
		instance.Spec.Clusters.Clusterset)
	if err != nil {
		return errors.New("mPolicy get clusterNamespaces failed," + err.Error())
	}
	if len(clusterNamespaces) == 0 {
		return errors.New("mPolicy get clusterNamespaces failed")
	}

	policyMap, err := getResourceAggregatePolicyMap(
		ctx,
		r.Client,
		common.NewNamespacedName(instance.GetNamespace(),
			instance.GetName()))
	if err != nil {
		return errors.New("mPolicy get clusterNamespaces failed," + err.Error())
	}

	for _, clusterNamespace := range clusterNamespaces {
		for _, ruleName := range instance.Spec.AggregateRules {
			// get rule
			rule, err := getAggregateRule(ctx, r.Client, ruleName, instance.GetNamespace())
			if err != nil {
				return errors.New("can not find rule," + err.Error())
			}

			policyMapKey := resourceAggregatePolicyMapKey(instance.GetNamespace(), instance.GetName(), rule.GetName())
			resourceAggregatePolicy, ok := policyMap[policyMapKey]
			if !ok {
				// create ResourceAggregatePolicy
				resourceAggregatePolicy = newResourceAggregatePolicy(clusterNamespace, rule, instance)
				err = r.Client.Create(ctx, resourceAggregatePolicy)
				if err != nil {
					return errors.New("create resourceAggregatePolicy failed," + err.Error())
				}
				continue
			}
			// update
			delete(policyMap, policyMapKey)
			policySpec := v1alpha1.ResourceAggregatePolicySpec{
				ResourceRef: rule.Spec.ResourceRef,
				Limit:       instance.Spec.Limit,
			}
			if reflect.DeepEqual(resourceAggregatePolicy.Spec, policySpec) {
				r.log.Info(fmt.Sprintf(
					"can not update resourceAggregatePolicy(%s:%s)",
					resourceAggregatePolicy.Namespace,
					resourceAggregatePolicy.Name))
				continue
			}
			resourceAggregatePolicy.Spec = policySpec
			err = r.Client.Update(ctx, resourceAggregatePolicy)
			if err != nil {
				return errors.New("update resourceAggregatePolicy failed," + err.Error())
			}
		}
	}

	if len(policyMap) == 0 {
		return nil
	}

	// delete
	for _, p := range policyMap {
		err = r.Client.Delete(ctx, p)
		if err != nil {
			return errors.New("delete resourceAggregatePolicy failed," + err.Error())
		}
	}

	return nil
}

func shouldChangePolicyLabels(instance *v1alpha1.MultiClusterResourceAggregatePolicy) bool {
	if len(instance.Spec.AggregateRules) == 0 {
		return false
	}
	currentLabels := getPolicyRuleLabels(instance)
	if len(currentLabels) == 0 {
		return true
	}
	existLabels := shouldExistLabels(instance)
	return !reflect.DeepEqual(existLabels, currentLabels)
}
func (r *Reconciler) addPolicyLabels(ctx context.Context, instance *v1alpha1.MultiClusterResourceAggregatePolicy) error {
	currentLabels := getPolicyRuleLabels(instance)
	existLabels := shouldExistLabels(instance)

	instance.SetLabels(replaceLabels(instance.GetLabels(), currentLabels, existLabels))
	return r.Client.Update(ctx, instance)
}

func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.MultiClusterResourceAggregatePolicy{}).
		Complete(r)
}

func Setup(mgr ctrl.Manager, controllerCommon controllerCommon.Args) error {
	reconciler := Reconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
		log:    logf.Log.WithName("multi_resource_aggregate_policy_controller"),
	}
	reconciler.Recorder = mgr.GetEventRecorderFor("stellaris-core")
	return reconciler.SetupWithManager(mgr)
}

func getPolicyRuleLabels(policy *v1alpha1.MultiClusterResourceAggregatePolicy) map[string]string {
	labels := map[string]string{}
	for k, v := range policy.GetLabels() {
		if strings.HasPrefix(k, managerCommon.AggregateRuleLabelName) {
			labels[k] = v
		}
	}
	return labels
}

func shouldExistLabels(policy *v1alpha1.MultiClusterResourceAggregatePolicy) map[string]string {
	existLabels := map[string]string{}
	for _, ruleName := range policy.Spec.AggregateRules {
		existLabels[managerCommon.AggregateRuleLabelName+"."+ruleName] = "1"
	}
	return existLabels
}

func replaceLabels(policyLabels, removeLabels, addLabels map[string]string) map[string]string {
	if len(policyLabels) == 0 || len(removeLabels) == 0 {
		return addLabels
	}
	if reflect.DeepEqual(policyLabels, removeLabels) {
		return addLabels
	}
	for removeKey := range removeLabels {
		delete(policyLabels, removeKey)
	}
	for addKey, addValue := range addLabels {
		policyLabels[addKey] = addValue
	}
	return policyLabels
}
