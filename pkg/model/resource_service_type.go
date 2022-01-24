package model

import "harmonycloud.cn/stellaris/pkg/apis/multicluster/v1alpha1"

type ResourceRequest struct {
	ClusterResourceStatusList []ClusterResourceStatus
}

type ClusterResourceStatus struct {
	Name      string
	Namespace string
	Status    string
}

type SyncResourceResponse struct {
	ClusterResourceList []*v1alpha1.ClusterResource
}
