package model

import "harmonycloud.cn/stellaris/pkg/apis/multicluster/v1alpha1"

type SyncAggregateResourceModel struct {
	RuleList   []*v1alpha1.MultiClusterResourceAggregateRule `json:"ruleList,omitempty"`
	PolicyList []*v1alpha1.ResourceAggregatePolicy           `json:"policyList,omitempty"`
}
