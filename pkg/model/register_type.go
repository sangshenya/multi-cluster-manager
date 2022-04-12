package model

import "harmonycloud.cn/stellaris/pkg/apis/multicluster/v1alpha1"

type RegisterRequest struct {
	Addons []AddonsData `json:"addons"`
	Token  string       `json:"token"`
}

type RegisterResponse struct {
	ClusterResources []v1alpha1.ClusterResource                   `json:"clusterResources"`
	Policies         []v1alpha1.ResourceAggregatePolicy           `json:"policies"`
	Rules            []v1alpha1.MultiClusterResourceAggregateRule `json:"rules"`
}

func (r *RegisterResponse) IsEmpty() bool {
	if len(r.ClusterResources) == 0 &&
		len(r.Policies) == 0 &&
		len(r.Rules) == 0 {
		return true
	}
	return false
}
