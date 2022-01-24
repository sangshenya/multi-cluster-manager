package handler

import (
	"encoding/json"
	"fmt"

	"harmonycloud.cn/stellaris/pkg/agent/send"

	"harmonycloud.cn/stellaris/config"

	"harmonycloud.cn/stellaris/pkg/model"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var heartbeatLog = logf.Log.WithName("agent_heartbeat")

func RecvHeartbeatResponse(res *config.Response) {
	heartbeatLog.Info(res.String(), "get response info form heartbeat")
	if res.Type != model.HeartbeatSuccess.String() {
		heartbeatLog.Info(fmt.Sprintf("send heartbeat request failed, error: %s", res.Body))
		return
	}
	heartbeatWithChange := &model.HeartbeatWithChangeRequest{}
	err := json.Unmarshal([]byte(res.Body), heartbeatWithChange)
	if err == nil {
		send.SetLastHeartbeat(heartbeatWithChange)
	}
}
