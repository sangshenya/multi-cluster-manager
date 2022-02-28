package aggregate

import (
	"context"

	managerCommon "harmonycloud.cn/stellaris/pkg/common"
	"harmonycloud.cn/stellaris/pkg/util/common"
	cueRender "harmonycloud.cn/stellaris/pkg/util/cur-render"

	"harmonycloud.cn/stellaris/pkg/agent/aggregate/match"

	agentconfig "harmonycloud.cn/stellaris/pkg/agent/config"
	"harmonycloud.cn/stellaris/pkg/apis/multicluster/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func aggregateResource(ctx context.Context, ruleList []v1alpha1.MultiClusterResourceAggregateRule, policyList []v1alpha1.ResourceAggregatePolicy) {
	resourceObjectList := &unstructured.UnstructuredList{}
	err := agentconfig.AgentConfig.ControllerClient.List(ctx, resourceObjectList)
	if err != nil {
		return
	}
	for _, policy := range policyList {
		rule := getMatchRule(ruleList, policy)
		for _, resourceObject := range resourceObjectList.Items {
			if !match.IsTargetResourceWithConfig(ctx, &resourceObject, &policy.Spec) {
				continue
			}
			resourceData, err := cueRender.RenderCue(&resourceObject, rule.Spec.Rule.Cue, "")
			if err != nil {
				continue
			}
		}
	}

}

func getMatchRule(ruleList []v1alpha1.MultiClusterResourceAggregateRule, policy v1alpha1.ResourceAggregatePolicy) *v1alpha1.MultiClusterResourceAggregateRule {
	for _, rule := range ruleList {
		policyLabels := policy.GetLabels()
		ruleNamespaced, ok := policyLabels[managerCommon.AggregateRuleLabelName]
		if ok && ruleNamespaced == common.NewNamespacedName(policy.Namespace, policy.Name).String() {
			return &rule
		}
	}
	return nil
}

func newResourceObject(gvk *metav1.GroupVersionKind) *unstructured.UnstructuredList {
	resourceObject := &unstructured.UnstructuredList{}
	resourceObject.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   gvk.Group,
		Version: gvk.Version,
		Kind:    gvk.Kind + "List",
	})
	return resourceObject
}
