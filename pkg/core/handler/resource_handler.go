package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

	"harmonycloud.cn/stellaris/pkg/apis/multicluster/v1alpha1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"harmonycloud.cn/stellaris/pkg/util/core"

	"harmonycloud.cn/stellaris/config"
	table "harmonycloud.cn/stellaris/pkg/core/stream"
	"harmonycloud.cn/stellaris/pkg/model"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var resourceHandlerLog = logf.Log.WithName("resource_handler")

// receive request form agent
func (s *CoreServer) Resource(req *config.Request, stream config.Channel_EstablishServer) {
	resourceHandlerLog.Info(fmt.Sprintf("receive grpc request for resource, cluster:%s", req.ClusterName))
	// convert data to cluster cr
	data := &model.ResourceRequest{}
	err := json.Unmarshal([]byte(req.Body), data)
	if err != nil {
		resourceHandlerLog.Error(err, "unmarshal data error")
		core.SendErrResponse(req.ClusterName, model.ResourceStatusUpdateFailed, err, stream)
	}
	// resource status update
	err = s.syncClusterResourceStatus(data.ClusterResourceStatusList)
	if err != nil {
		core.SendErrResponse(req.ClusterName, model.ResourceStatusUpdateFailed, err, stream)
	}

	core.SendResponse(&config.Response{
		Type:        model.ResourceStatusUpdateSuccess.String(),
		ClusterName: req.ClusterName,
		Body:        "",
	}, stream)
}

func (s *CoreServer) syncClusterResourceStatus(statusList []model.ClusterResourceStatus) error {
	ctx := context.Background()
	for _, item := range statusList {
		clusterResource, err := s.mClient.MulticlusterV1alpha1().ClusterResources(item.Namespace).Get(ctx, item.Name, metav1.GetOptions{})
		if err != nil {
			resourceHandlerLog.Error(err, fmt.Sprintf("get clusterResource(%s:%s) failed", item.Namespace, item.Name))
			return err
		}
		status := &v1alpha1.ClusterResourceStatus{}
		err = json.Unmarshal([]byte(item.Status), status)
		if err != nil {
			resourceHandlerLog.Error(err, fmt.Sprintf("unmarshal clusterResource(%s:%s) status failed", item.Namespace, item.Name))
			return err
		}
		ma, _ := json.Marshal(clusterResource.Status)
		resourceHandlerLog.Info(fmt.Sprintf("status:(%s), clusterResourceStatus(%s)", item.Status, string(ma)))
		if reflect.DeepEqual(clusterResource.Status, *status) {
			resourceHandlerLog.Info(fmt.Sprintf("clusterResource(%s:%s) status is no changed", item.Namespace, item.Name))
			continue
		}
		clusterResource.Status = *status
		_, err = s.mClient.MulticlusterV1alpha1().ClusterResources(item.Namespace).UpdateStatus(ctx, clusterResource, metav1.UpdateOptions{})
		if err != nil {
			resourceHandlerLog.Error(err, fmt.Sprintf("update clusterResource(%s:%s) status failed", item.Namespace, item.Name))
			return err
		}
	}
	return nil
}

// send request to agent
func SendResourceToAgent(clusterName string, resourceResponse *config.Response) error {
	stream := table.FindStream(clusterName)
	if stream == nil {
		err := errors.New(fmt.Sprintf("can not find agent(%s) stream", clusterName))
		resourceHandlerLog.Error(err, "find agent stream failed")
		return err
	}
	err := stream.Stream.Send(resourceResponse)
	if err != nil {
		resourceHandlerLog.Error(err, "send resource to agent failed")
		return err
	}
	return nil
}

func NewResourceResponse(resType model.ServiceResponseType, clusterName string, body string) (*config.Response, error) {
	if len(clusterName) == 0 || len(body) == 0 {
		return nil, errors.New("clusterName or body is empty")
	}
	return &config.Response{
		Type:        resType.String(),
		ClusterName: clusterName,
		Body:        body,
	}, nil
}
