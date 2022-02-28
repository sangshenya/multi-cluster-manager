package model

import (
	"harmonycloud.cn/stellaris/pkg/apis/multicluster/v1alpha1"
	"harmonycloud.cn/stellaris/pkg/util/common"
)

type SyncAggregateResourceModel struct {
	RuleList   []v1alpha1.MultiClusterResourceAggregateRule `json:"ruleList,omitempty"`
	PolicyList []v1alpha1.ResourceAggregatePolicy           `json:"policyList,omitempty"`
}

type SyncAggregateResourceType string

const (
	UnknownType            SyncAggregateResourceType = "Unknown"
	SyncResource           SyncAggregateResourceType = "Sync"
	UpdateOrCreateResource SyncAggregateResourceType = "UpdateOrCreate"
	DeleteResource         SyncAggregateResourceType = "Delete"
)

type AggregateResourceDataModel struct {
	ResourceAggregatePolicy           *common.NamespacedName    `json:"resourceAggregatePolicy"`
	MultiClusterResourceAggregateRule *common.NamespacedName    `json:"multiClusterResourceAggregateRule"`
	TargetResourceData                []TargetResourceDataModel `json:"targetResourceData"`
}

type TargetResourceDataModel struct {
	Namespace        string              `json:"namespace"`
	ResourceInfoList []ResourceDataModel `json:"resourceInfoList"`
}

type ResourceDataModel struct {
	Name         string `json:"name"`
	ResourceData []byte `json:"resourceData"`
}
