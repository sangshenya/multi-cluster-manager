package model

type RegisterRequest struct {
	Addons []Addon `json:"addons"`
}

type RegisterResponse struct {
	ClusterResources                      []string `json:"clusterResources"`
	MultiClusterResourceAggregatePolicies []string `json:"multiClusterResourceAggregatePolicies"`
	MultiClusterResourceAggregateRules    []string `json:"multiClusterResourceAggregateRules"`
}

func (r *RegisterResponse) IsEmpty() bool {
	if len(r.ClusterResources) == 0 && len(r.MultiClusterResourceAggregateRules) == 0 && len(r.MultiClusterResourceAggregatePolicies) == 0 {
		return true
	}
	return false
}

type Addon struct {
	Name       string      `json:"name"`
	Properties interface{} `json:"properties,omitempty"`
}
