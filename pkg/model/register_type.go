package model

type RegisterRequest struct {
	Addons []Addon
}

type RegisterResponse struct {
	ClusterResources                      []string
	MultiClusterResourceAggregatePolicies []string
	MultiClusterResourceAggregateRules    []string
}

func (r *RegisterResponse) IsEmpty() bool {
	if len(r.ClusterResources) == 0 && len(r.MultiClusterResourceAggregateRules) == 0 && len(r.MultiClusterResourceAggregatePolicies) == 0 {
		return true
	}
	return false
}

type Addon struct {
	Name       string
	Properties map[string]string
}
