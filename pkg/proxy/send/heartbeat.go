package send

import (
	"context"
	"errors"
	"sync"
	"time"

	"harmonycloud.cn/stellaris/config"

	clusterHealth "harmonycloud.cn/stellaris/pkg/common/cluster-health"
	"harmonycloud.cn/stellaris/pkg/model"
	"harmonycloud.cn/stellaris/pkg/proxy/addons"
	"harmonycloud.cn/stellaris/pkg/proxy/condition"
	proxy_cfg "harmonycloud.cn/stellaris/pkg/proxy/config"
	proxy_stream "harmonycloud.cn/stellaris/pkg/proxy/stream"
	"harmonycloud.cn/stellaris/pkg/utils/common"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var heartbeatLog = logf.Log.WithName("proxy_send_heartbeat")

type HeartbeatObject struct {
	LastHeartbeat *model.HeartbeatWithChangeRequest
}

var heartbeat *HeartbeatObject
var onec sync.Once

func HeartbeatStart() {
	onec.Do(func() {
		heartbeat = &HeartbeatObject{}
		heartbeat.start()
	})
}

func (heartbeat *HeartbeatObject) start() {
	for {
		time.Sleep(proxy_cfg.ProxyConfig.Cfg.HeartbeatPeriod)
		heartbeatLog.Info("start send heartbeat to core")

		// get addons
		addonsList := addons.LoadAddon(context.Background(), proxy_cfg.ProxyConfig.ControllerClient)
		// get condition
		conditions := condition.GetProxyCondition()
		// CHECK HEALTH
		_, healthy := clusterHealth.GetClusterHealthStatus(proxy_cfg.ProxyConfig.ProxyClient)

		heartbeatWithChange := newHeartbeat(addonsList, conditions, healthy)
		request, err := common.GenerateRequest(model.Heartbeat.String(), heartbeatWithChange, proxy_cfg.ProxyConfig.Cfg.ClusterName)
		if err != nil {
			heartbeatLog.Error(err, "create Heartbeat request failed")
			continue
		}
		err = sendHeartbeatRequestToCore(request)
		if err != nil {
			heartbeatLog.Error(err, "send Heartbeat request failed")
			continue
		}
	}
}

func newHeartbeat(addons []model.AddonsData, conditions []model.Condition, healthy bool) *model.HeartbeatWithChangeRequest {
	heartbeatWithChange := &model.HeartbeatWithChangeRequest{}
	if len(addons) > 0 {
		heartbeatWithChange.Addons = addons
	}
	heartbeatWithChange.Conditions = conditions
	heartbeatWithChange.Healthy = healthy
	return heartbeatWithChange
}

func sendHeartbeatRequestToCore(request *config.Request) error {
	stream := proxy_stream.GetConnection()
	if stream == nil {
		return errors.New("new stream failed")
	}
	if err := stream.Send(request); err != nil {
		return err
	}
	return nil
}

func SetLastHeartbeat(request *model.HeartbeatWithChangeRequest) {
	heartbeat.LastHeartbeat = request
}
