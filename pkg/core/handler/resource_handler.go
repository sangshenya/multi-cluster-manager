package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

	table "harmonycloud.cn/stellaris/pkg/core/stream"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"harmonycloud.cn/stellaris/pkg/utils/core"

	"harmonycloud.cn/stellaris/config"
	"harmonycloud.cn/stellaris/pkg/model"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var resourceHandlerLog = logf.Log.WithName("resource_handler")

// receive request form proxy
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
		if reflect.DeepEqual(clusterResource.Status, item.Status) {
			resourceHandlerLog.Info(fmt.Sprintf("clusterResource(%s:%s) status is no changed", item.Namespace, item.Name))
			continue
		}
		clusterResource.Status = item.Status
		_, err = s.mClient.MulticlusterV1alpha1().ClusterResources(item.Namespace).UpdateStatus(ctx, clusterResource, metav1.UpdateOptions{})
		if err != nil {
			resourceHandlerLog.Error(err, fmt.Sprintf("update clusterResource(%s:%s) status failed", item.Namespace, item.Name))
			return err
		}
	}
	return nil
}

// send request to proxy
func SendResourceToProxy(clusterName string, resourceResponse *config.Response) error {
	resourceHandlerLog.Info(fmt.Sprintf("start to send resource request to proxy"))
	stream := table.FindStream(clusterName)
	if stream == nil {
		err := errors.New(fmt.Sprintf("cannot find proxy(%s) stream", clusterName))
		resourceHandlerLog.Error(err, "find proxy stream failed")
		return err
	}
	err := stream.Stream.Send(resourceResponse)
	if err != nil {
		resourceHandlerLog.Error(err, "send resource to proxy failed")
		return err
	}
	resourceHandlerLog.Info(fmt.Sprintf("send resource request to proxy success"))
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
