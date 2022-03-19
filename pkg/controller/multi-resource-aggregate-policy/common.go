package multi_resource_aggregate_policy

import (
	"context"

	"k8s.io/apimachinery/pkg/labels"

	"harmonycloud.cn/stellaris/pkg/utils/common"

	managerCommon "harmonycloud.cn/stellaris/pkg/common"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/types"

	"harmonycloud.cn/stellaris/pkg/apis/multicluster/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func getAggregateRule(ctx context.Context, clientSet client.Client, ruleName, ruleNamespace string) (*v1alpha1.MultiClusterResourceAggregateRule, error) {
	rule := &v1alpha1.MultiClusterResourceAggregateRule{}
	err := clientSet.Get(ctx, types.NamespacedName{
		Namespace: ruleNamespace,
		Name:      ruleName,
	}, rule)
	if err != nil {
		return nil, err
	}
	return rule, nil
}

func getResourceAggregatePolicyMap(
	ctx context.Context,
	clientSet client.Client,
	mPolicyName common.NamespacedName) (map[string]*v1alpha1.ResourceAggregatePolicy, error) {
	policyMap := map[string]*v1alpha1.ResourceAggregatePolicy{}
	policyList, err := getResourceAggregatePolicyList(ctx, clientSet, mPolicyName)
	if err != nil {
		return policyMap, err
	}
	for _, policy := range policyList.Items {
		ns, ok := policy.GetLabels()[managerCommon.ParentResourceNamespaceLabelName]
		if !ok || len(ns) == 0 {
			continue
		}
		mPolicyNameStr, ok := policy.GetLabels()[managerCommon.MultiAggregatePolicyLabelName]
		if !ok || len(mPolicyNameStr) == 0 {
			continue
		}
		ruleNameStr, ok := policy.GetLabels()[managerCommon.AggregateRuleLabelName]
		if !ok || len(ruleNameStr) == 0 {
			continue
		}
		policyMap[resourceAggregatePolicyMapKey(ns, mPolicyNameStr, ruleNameStr)] = &policy
	}
	return policyMap, nil
}

func resourceAggregatePolicyMapKey(ns, mPolicyNameStr, ruleNameStr string) string {
	return ns + "." + mPolicyNameStr + "." + ruleNameStr
}

func getResourceAggregatePolicyList(ctx context.Context, clientSet client.Client, name common.NamespacedName) (*v1alpha1.ResourceAggregatePolicyList, error) {
	policyList := &v1alpha1.ResourceAggregatePolicyList{}
	set := labels.Set{
		managerCommon.MultiAggregatePolicyLabelName:    name.Name,
		managerCommon.ParentResourceNamespaceLabelName: name.Namespace,
	}
	err := clientSet.List(ctx, policyList, &client.ListOptions{
		LabelSelector: labels.SelectorFromSet(set),
	})
	return policyList, err
}

func resourceAggregatePolicyName(resourceRef *metav1.GroupVersionKind) string {
	return managerCommon.GvkLabelString(resourceRef)
}

func newResourceAggregatePolicy(
	clusterNamespace string,
	rule *v1alpha1.MultiClusterResourceAggregateRule,
	mPolicy *v1alpha1.MultiClusterResourceAggregatePolicy) *v1alpha1.ResourceAggregatePolicy {
	resourceAggregatePolicy := &v1alpha1.ResourceAggregatePolicy{}
	resourceAggregatePolicy.SetName(resourceAggregatePolicyName(rule.Spec.ResourceRef))
	resourceAggregatePolicy.SetNamespace(clusterNamespace)
	// info
	resourceAggregatePolicy.Spec.ResourceRef = rule.Spec.ResourceRef
	resourceAggregatePolicy.Spec.Limit = mPolicy.Spec.Limit
	// label
	policyLabels := map[string]string{}
	policyLabels[managerCommon.ParentResourceNamespaceLabelName] = mPolicy.Namespace
	policyLabels[managerCommon.MultiAggregatePolicyLabelName] = mPolicy.Name
	policyLabels[managerCommon.AggregateRuleLabelName] = rule.Name
	resourceAggregatePolicy.SetLabels(policyLabels)

	return resourceAggregatePolicy
}
