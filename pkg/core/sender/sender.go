package sender

import (
	"errors"
	"fmt"

	"harmonycloud.cn/stellaris/config"
	table "harmonycloud.cn/stellaris/pkg/core/stream"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var senderLog = logf.Log.WithName("core_sender")

// send request to agent
func SendResponseToAgent(resourceResponse *config.Response) error {
	senderLog.Info(fmt.Sprintf("start to send resource request to agent"))
	stream := table.FindStream(resourceResponse.ClusterName)
	if stream == nil {
		err := errors.New(fmt.Sprintf("can not find agent(%s) stream", resourceResponse.ClusterName))
		senderLog.Error(err, "find agent stream failed")
		return err
	}
	err := stream.Stream.Send(resourceResponse)
	if err != nil {
		senderLog.Error(err, fmt.Sprintf("send responstType(%s) resource(%s) to agent(%s) failed", resourceResponse.Type, resourceResponse.Body, resourceResponse.ClusterName))
		return err
	}
	senderLog.Info(fmt.Sprintf("send resource request to agent success"))
	return nil
}
