package handler

import (
	"encoding/json"
	"fmt"

	"harmonycloud.cn/stellaris/config"
	"harmonycloud.cn/stellaris/pkg/apis/multicluster/v1alpha1"
	"harmonycloud.cn/stellaris/pkg/model"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var resourceLog = logf.Log.WithName("agent_resource")

func SendSyncResourceRequest() {
	resourceLog.Info(fmt.Sprintf("start send resource request to core"))
}

func NewResourceRequest(clusterResourceList []*v1alpha1.ClusterResource, clusterName string) (*config.Request, error) {
	request := &model.ResourceRequest{}
	for _, clusterResource := range clusterResourceList {
		status := model.ClusterResourceStatus{}
		status.Name = clusterResource.Name
		status.Namespace = clusterResource.Namespace
		data, err := json.Marshal(&clusterResource.Status)
		if err != nil {
			return nil, err
		}
		status.Status = string(data)
		request.ClusterResourceStatusList = append(request.ClusterResourceStatusList, status)
	}
	requestData, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	return &config.Request{
		Type:        model.Resource.String(),
		ClusterName: clusterName,
		Body:        string(requestData),
	}, nil
}
