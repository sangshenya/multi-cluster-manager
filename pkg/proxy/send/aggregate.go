package send

import (
	"errors"
	"fmt"

	"harmonycloud.cn/stellaris/config"
	"harmonycloud.cn/stellaris/pkg/model"
	proxyStream "harmonycloud.cn/stellaris/pkg/proxy/stream"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var aggregateLog = logf.Log.WithName("proxy_send_aggregate")

func NewAggregateRequest(clusterName string, body string) (*config.Request, error) {
	if len(clusterName) == 0 || len(body) == 0 {
		return nil, errors.New("clusterName or body is empty")
	}
	return &config.Request{
		Type:        model.Aggregate.String(),
		ClusterName: clusterName,
		Body:        body,
	}, nil
}

func SendSyncAggregateRequest(request *config.Request) error {
	resourceLog.Info(fmt.Sprintf("start send aggregate request to core, request Data:%s", request.Body))
	stream := proxyStream.GetConnection()
	var err error
	if stream == nil {
		err = errors.New("new stream failed")
		aggregateLog.Error(err, "send aggregate request")
		return err
	}
	err = stream.Send(request)
	if err != nil {
		aggregateLog.Error(err, "send aggregate request failed")
		return err
	}
	return nil
}