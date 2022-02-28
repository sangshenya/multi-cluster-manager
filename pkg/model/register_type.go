package model

import "harmonycloud.cn/stellaris/pkg/apis/multicluster/v1alpha1"

type RegisterRequest struct {
	Addons []Addon `json:"addons"`
}

type RegisterResponse struct {
	ClusterResources                   []v1alpha1.ClusterResource                   `json:"clusterResources"`
	ResourceAggregatePolicies          []v1alpha1.ResourceAggregatePolicy           `json:"resourceAggregatePolicies"`
	MultiClusterResourceAggregateRules []v1alpha1.MultiClusterResourceAggregateRule `json:"multiClusterResourceAggregateRules"`
}

func (r *RegisterResponse) IsEmpty() bool {
	if len(r.ClusterResources) == 0 && len(r.MultiClusterResourceAggregateRules) == 0 && len(r.ResourceAggregatePolicies) == 0 {
		return true
	}
	return false
}

type Addon struct {
	Name       string      `json:"name"`
	Properties interface{} `json:"properties,omitempty"`
}
