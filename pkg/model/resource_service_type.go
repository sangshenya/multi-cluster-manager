package model

type ResourceRequest struct {
	ClusterResourceStatusList []ClusterResourceStatus
}

type ClusterResourceStatus struct {
	Name      string
	Namespace string
	Status    string
}
