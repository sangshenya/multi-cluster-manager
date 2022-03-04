package model

import (
	"harmonycloud.cn/stellaris/pkg/apis/multicluster/v1alpha1"
	"harmonycloud.cn/stellaris/pkg/utils/common"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

type AggregateResourceDataModelList struct {
	List []AggregateResourceDataModel `json:"list"`
}

type AggregateResourceDataModel struct {
	ResourceAggregatePolicy           common.NamespacedName     `json:"resourceAggregatePolicy"`
	MultiClusterResourceAggregateRule common.NamespacedName     `json:"multiClusterResourceAggregateRule"`
	ResourceRef                       *metav1.GroupVersionKind  `json:"resourceRef"`
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
