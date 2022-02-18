package multi_resource_aggregate_policy

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"harmonycloud.cn/stellaris/pkg/util/common"

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
	log    logr.Logger
	Scheme *runtime.Scheme
}

func (r *Reconciler) Reconcile(ctx context.Context, request ctrl.Request) (ctrl.Result, error) {
	r.log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	r.log.Info("Reconciling MultiClusterResourceAggregatePolicy")
	// get resource
	instance := &v1alpha1.MultiClusterResourceAggregatePolicy{}
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

	// add labels
	if shouldChangePolicyLabels(instance) {
		return r.addPolicyLabels(ctx, instance)
	}

	return r.syncResourceAggregatePolicy(ctx, instance)
}

// sync ResourceAggregatePolicy
func (r *Reconciler) syncResourceAggregatePolicy(ctx context.Context, instance *v1alpha1.MultiClusterResourceAggregatePolicy) (ctrl.Result, error) {
	if len(instance.Spec.AggregateRules) == 0 || instance.Spec.Clusters == nil {
		return ctrl.Result{}, nil
	}
	clusterNamespaces, err := controllerCommon.GetClusterNamespaces(ctx, r.Client, instance.Spec.Clusters.ClusterType, instance.Spec.Clusters.Clusters, instance.Spec.Clusters.Clusterset)
	if err != nil {
		r.log.Error(err, fmt.Sprintf("mPolicy(%s:%s) get clusterNamespaces failed", instance.GetNamespace(), instance.GetName()))
		return controllerCommon.ReQueueResult(err)
	}
	if len(clusterNamespaces) <= 0 {
		err = errors.New("can not find clusterNamespace")
		r.log.Error(err, fmt.Sprintf("mPolicy(%s:%s) get clusterNamespaces failed", instance.GetNamespace(), instance.GetName()))
		return controllerCommon.ReQueueResult(err)
	}

	policyMap, err := getResourceAggregatePolicyMap(ctx, r.Client, common.NewNamespacedName(instance.GetNamespace(), instance.GetName()))
	if err != nil {
		r.log.Error(err, "get ResourceAggregatePolicy failed")
		return controllerCommon.ReQueueResult(err)
	}

	for _, clusterNamespace := range clusterNamespaces {
		for _, ruleName := range instance.Spec.AggregateRules {
			// get rule
			rule, err := getPolicyRule(ctx, r.Client, ruleName, instance.GetNamespace())
			if err != nil {
				r.log.Error(err, fmt.Sprintf("policyRule(%s:%s) can not find", instance.GetNamespace(), ruleName))
				continue
			}

			policyMapKey := getResourceAggregatePolicyMapKey(common.NewNamespacedName(instance.GetNamespace(), instance.GetName()), common.NewNamespacedName(rule.GetNamespace(), rule.GetName()))
			resourceAggregatePolicy, ok := policyMap[policyMapKey]
			if !ok {
				// create ResourceAggregatePolicy
				resourceAggregatePolicy = newResourceAggregatePolicy(clusterNamespace, rule, instance)
				err = r.Client.Create(ctx, resourceAggregatePolicy)
				if err != nil {
					r.log.Error(err, fmt.Sprintf("create resourceAggregatePolicy(%s:%s) failed", clusterNamespace, resourceAggregatePolicy.Name))
					return controllerCommon.ReQueueResult(err)
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
				r.log.Info(fmt.Sprintf("can not update resourceAggregatePolicy(%s:%s)", resourceAggregatePolicy.Namespace, resourceAggregatePolicy.Name))
				continue
			}
			resourceAggregatePolicy.Spec = policySpec
			err = r.Client.Update(ctx, resourceAggregatePolicy)
			if err != nil {
				r.log.Error(err, fmt.Sprintf("update resourceAggregatePolicy(%s:%s) failed", clusterNamespace, resourceAggregatePolicy.Name))
				return controllerCommon.ReQueueResult(err)
			}
		}
	}

	if len(policyMap) <= 0 {
		return ctrl.Result{}, nil
	}

	// delete
	for _, p := range policyMap {
		err = r.Client.Delete(ctx, p)
		if err != nil {
			r.log.Error(err, fmt.Sprintf("delete resourceAggregatePolicy(%s:%s) failed", p.Namespace, p.Name))
			return controllerCommon.ReQueueResult(err)
		}
	}

	return ctrl.Result{}, nil
}

// add labels
func shouldChangePolicyLabels(instance *v1alpha1.MultiClusterResourceAggregatePolicy) bool {
	if len(instance.Spec.AggregateRules) <= 0 {
		return false
	}
	currentLabels := getPolicyRuleLabels(instance)
	if len(currentLabels) <= 0 {
		return true
	}
	existLabels := shouldExistLabels(instance)
	if reflect.DeepEqual(existLabels, currentLabels) {
		return false
	}
	return true
}
func (r *Reconciler) addPolicyLabels(ctx context.Context, instance *v1alpha1.MultiClusterResourceAggregatePolicy) (ctrl.Result, error) {
	currentLabels := getPolicyRuleLabels(instance)
	existLabels := shouldExistLabels(instance)

	instance.SetLabels(replaceLabels(instance.GetLabels(), currentLabels, existLabels))
	err := r.Client.Update(ctx, instance)
	if err != nil {
		return controllerCommon.ReQueueResult(err)
	}
	return ctrl.Result{}, nil
}

func (r *Reconciler) addFinalizer(ctx context.Context, instance *v1alpha1.MultiClusterResourceAggregatePolicy) (ctrl.Result, error) {
	err := controllerCommon.AddFinalizer(ctx, r.Client, instance)
	if err != nil {
		r.log.Error(err, fmt.Sprintf("append finalizer filed, resource(%s)", instance.Name))
		return controllerCommon.ReQueueResult(err)
	}
	return ctrl.Result{}, nil
}

func (r *Reconciler) removeFinalizer(ctx context.Context, instance *v1alpha1.MultiClusterResourceAggregatePolicy) (ctrl.Result, error) {
	err := controllerCommon.RemoveFinalizer(ctx, r.Client, instance)
	if err != nil {
		r.log.Error(err, fmt.Sprintf("delete finalizer filed, resource(%s)", instance.Name))
		return controllerCommon.ReQueueResult(err)
	}
	return ctrl.Result{}, nil
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
	if len(policyLabels) <= 0 || len(removeLabels) <= 0 {
		return addLabels
	}
	if reflect.DeepEqual(policyLabels, removeLabels) {
		return addLabels
	}
	for removeKey, _ := range removeLabels {
		delete(policyLabels, removeKey)
	}
	for addKey, addValue := range addLabels {
		policyLabels[addKey] = addValue
	}
	return policyLabels
}
