package model

import "harmonycloud.cn/stellaris/pkg/apis/multicluster/v1alpha1"

type ResourceRequest struct {
	ClusterResourceStatusList []ClusterResourceStatus `json:"clusterResourceStatusList"`
}

type ClusterResourceStatus struct {
	Name      string                         `json:"name"`
	Namespace string                         `json:"namespace"`
	Status    v1alpha1.ClusterResourceStatus `json:"status"`
}

type SyncResourceResponse struct {
	ClusterResourceList []*v1alpha1.ClusterResource `json:"clusterResourceList"`
}
