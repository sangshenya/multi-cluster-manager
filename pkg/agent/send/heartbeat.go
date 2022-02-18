package send

import (
	"fmt"
	"reflect"
	"sync"
	"time"

	"harmonycloud.cn/stellaris/pkg/agent/addons"
	"harmonycloud.cn/stellaris/pkg/agent/condition"
	agentconfig "harmonycloud.cn/stellaris/pkg/agent/config"
	agentStream "harmonycloud.cn/stellaris/pkg/agent/stream"
	clusterHealth "harmonycloud.cn/stellaris/pkg/common/cluster-health"
	"harmonycloud.cn/stellaris/pkg/model"
	"harmonycloud.cn/stellaris/pkg/util/agent"
	"harmonycloud.cn/stellaris/pkg/util/common"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var heartbeatLog = logf.Log.WithName("agent_send_heartbeat")

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
		time.Sleep(agentconfig.AgentConfig.Cfg.HeartbeatPeriod)
		heartbeatLog.Info(fmt.Sprintf("start send heartbeat to core"))

		// get addons
		addonsInfo := heartbeat.getAddon()
		// get condition
		conditions := condition.GetAgentCondition()
		// CHECK HEALTH
		_, healthy := clusterHealth.GetClusterHealthStatus(agentconfig.AgentConfig.AgentClient)

		heartbeatWithChange := &model.HeartbeatWithChangeRequest{}
		if !(heartbeat.LastHeartbeat != nil && isEqualAddons(addonsInfo, heartbeat.LastHeartbeat.Addons)) {
			heartbeatWithChange.Addons = addonsInfo
		}
		heartbeatWithChange.Conditions = conditions
		heartbeatWithChange.Healthy = healthy
		request, err := common.GenerateRequest(model.Heartbeat.String(), heartbeatWithChange, agentconfig.AgentConfig.Cfg.ClusterName)
		if err != nil {
			heartbeatLog.Error(err, "create Heartbeat request failed")
			continue
		}

		stream := agentStream.GetConnection()
		if stream == nil {
			heartbeatLog.Error(err, "new stream failed")
			continue
		}
		if err = stream.Send(request); err != nil {
			heartbeatLog.Error(err, "send request failed")
			continue
		}
	}
}

func SetLastHeartbeat(request *model.HeartbeatWithChangeRequest) {
	heartbeat.LastHeartbeat = request
}

func (heartbeat *HeartbeatObject) getAddon() []model.Addon {
	var addonsInfo []model.Addon
	if len(agentconfig.AgentConfig.Cfg.AddonPath) == 0 {
		return addonsInfo
	}
	addonConfig, err := agent.GetAddonConfig(agentconfig.AgentConfig.Cfg.AddonPath)
	if err != nil {
		return addonsInfo
	}
	addonsInfo = addons.LoadAddon(addonConfig)
	return addonsInfo
}

func isEqualAddons(new, old []model.Addon) bool {
	if len(old) == 0 {
		return false
	}
	if len(new) != len(old) {
		return false
	}
	return reflect.DeepEqual(getAddonMap(new), getAddonMap(old))
}

func getAddonMap(addonList []model.Addon) map[string]model.Addon {
	addonMap := map[string]model.Addon{}
	for _, item := range addonList {
		addonMap[item.Name] = item
	}
	return addonMap
}
