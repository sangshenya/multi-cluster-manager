package multi_resource_aggregate_policy

import (
	"context"

	"k8s.io/apimachinery/pkg/labels"

	"harmonycloud.cn/stellaris/pkg/util/common"

	managerCommon "harmonycloud.cn/stellaris/pkg/common"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/types"

	"harmonycloud.cn/stellaris/pkg/apis/multicluster/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func getPolicyRule(ctx context.Context, clientSet client.Client, ruleName, ruleNamespace string) (*v1alpha1.MultiClusterResourceAggregateRule, error) {
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

func getResourceAggregatePolicyMap(ctx context.Context, clientSet client.Client, mPolicyName common.NamespacedName) (map[string]*v1alpha1.ResourceAggregatePolicy, error) {
	policyMap := map[string]*v1alpha1.ResourceAggregatePolicy{}
	policyList, err := getResourceAggregatePolicyList(ctx, clientSet, mPolicyName)
	if err != nil {
		return policyMap, err
	}
	for _, policy := range policyList.Items {
		mPolicyNameStr, ok := policy.GetLabels()[managerCommon.MultiAggregatePolicyLabelName]
		if !ok || len(mPolicyNameStr) <= 0 {
			continue
		}
		ruleNameStr, ok := policy.GetLabels()[managerCommon.AggregateRuleLabelName]
		if !ok || len(ruleNameStr) <= 0 {
			continue
		}
		policyMap[resourceAggregatePolicyMapKey(mPolicyNameStr, ruleNameStr)] = &policy
	}
	return policyMap, nil
}

func resourceAggregatePolicyMapKey(mPolicyNameStr, ruleNameStr string) string {
	return mPolicyNameStr + "." + ruleNameStr
}

func getResourceAggregatePolicyMapKey(mPolicyName, ruleName common.NamespacedName) string {
	return resourceAggregatePolicyMapKey(mPolicyName.String(), ruleName.String())
}

func getResourceAggregatePolicyList(ctx context.Context, clientSet client.Client, name common.NamespacedName) (*v1alpha1.ResourceAggregatePolicyList, error) {
	policyList := &v1alpha1.ResourceAggregatePolicyList{}
	selector, err := labels.Parse(managerCommon.MultiAggregatePolicyLabelName + "=" + name.String())
	if err != nil {
		return nil, err
	}
	err = clientSet.List(ctx, policyList, &client.ListOptions{
		LabelSelector: selector,
	})
	return policyList, err
}

func resourceAggregatePolicyName(resourceRef *metav1.GroupVersionKind) string {
	return managerCommon.GvkLabelString(resourceRef)
}

func newResourceAggregatePolicy(policyNamespace string, rule *v1alpha1.MultiClusterResourceAggregateRule, mPolicy *v1alpha1.MultiClusterResourceAggregatePolicy) *v1alpha1.ResourceAggregatePolicy {
	resourceAggregatePolicy := &v1alpha1.ResourceAggregatePolicy{}
	resourceAggregatePolicy.SetName(resourceAggregatePolicyName(rule.Spec.ResourceRef))
	resourceAggregatePolicy.SetNamespace(policyNamespace)
	// info
	resourceAggregatePolicy.Spec.ResourceRef = rule.Spec.ResourceRef
	resourceAggregatePolicy.Spec.Limit = mPolicy.Spec.Limit
	// label
	policyLabels := map[string]string{}
	policyLabels[managerCommon.MultiAggregatePolicyLabelName] = common.NewNamespacedName(mPolicy.Namespace, mPolicy.Name).String()
	policyLabels[managerCommon.AggregateRuleLabelName] = common.NewNamespacedName(rule.Namespace, rule.Name).String()
	resourceAggregatePolicy.SetLabels(policyLabels)

	return resourceAggregatePolicy
}
