package common

import (
	"context"

	"harmonycloud.cn/stellaris/pkg/apis/multicluster/v1alpha1"
	multclusterclient "harmonycloud.cn/stellaris/pkg/client/clientset/versioned"
	managerCommon "harmonycloud.cn/stellaris/pkg/common"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetAggregateRuleListWithLabelSelector(ctx context.Context,
	clientSet *multclusterclient.Clientset,
	gvk *metav1.GroupVersionKind,
	ns string) (*v1alpha1.MultiClusterResourceAggregateRuleList, error) {
	targetGvkString := managerCommon.GvkLabelString(gvk)
	return clientSet.MulticlusterV1alpha1().MultiClusterResourceAggregateRules(ns).List(ctx, metav1.ListOptions{
		LabelSelector: managerCommon.AggregateResourceGvkLabelName + "=" + targetGvkString,
	})
}

func GetAggregatePolicyListWithLabelSelector(ctx context.Context,
	clientSet *multclusterclient.Clientset,
	ns string,
	ruleName NamespacedName) (*v1alpha1.ResourceAggregatePolicyList, error) {
	return clientSet.MulticlusterV1alpha1().ResourceAggregatePolicies(ns).List(ctx, metav1.ListOptions{
		LabelSelector: managerCommon.AggregateRuleLabelName + "=" + ruleName.Namespace + "." + ruleName.Name,
	})
}
