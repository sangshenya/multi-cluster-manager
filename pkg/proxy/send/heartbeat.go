package send

import (
	"fmt"
	"reflect"
	"sync"
	"time"

	clusterHealth "harmonycloud.cn/stellaris/pkg/common/cluster-health"
	"harmonycloud.cn/stellaris/pkg/model"
	"harmonycloud.cn/stellaris/pkg/proxy/addons"
	"harmonycloud.cn/stellaris/pkg/proxy/condition"
	proxy_cfg "harmonycloud.cn/stellaris/pkg/proxy/config"
	proxy_stream "harmonycloud.cn/stellaris/pkg/proxy/stream"
	"harmonycloud.cn/stellaris/pkg/utils/common"
	"harmonycloud.cn/stellaris/pkg/utils/proxy"
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
		heartbeatLog.Info(fmt.Sprintf("start send heartbeat to core"))

		// get addons
		addonsInfo := heartbeat.getAddon()
		// get condition
		conditions := condition.GetProxyCondition()
		// CHECK HEALTH
		_, healthy := clusterHealth.GetClusterHealthStatus(proxy_cfg.ProxyConfig.ProxyClient)

		heartbeatWithChange := &model.HeartbeatWithChangeRequest{}
		if !(heartbeat.LastHeartbeat != nil && isEqualAddons(addonsInfo, heartbeat.LastHeartbeat.Addons)) {
			heartbeatWithChange.Addons = addonsInfo
		}
		heartbeatWithChange.Conditions = conditions
		heartbeatWithChange.Healthy = healthy
		request, err := common.GenerateRequest(model.Heartbeat.String(), heartbeatWithChange, proxy_cfg.ProxyConfig.Cfg.ClusterName)
		if err != nil {
			heartbeatLog.Error(err, "create Heartbeat request failed")
			continue
		}

		stream := proxy_stream.GetConnection()
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

func (heartbeat *HeartbeatObject) getAddon() []model.AddonsData {
	var addonsInfo []model.AddonsData
	if len(proxy_cfg.ProxyConfig.Cfg.AddonPath) == 0 {
		return addonsInfo
	}
	addonConfig, err := proxy.GetAddonConfig(proxy_cfg.ProxyConfig.Cfg.AddonPath)
	if err != nil {
		return addonsInfo
	}
	addonsInfo = addons.LoadAddon(addonConfig)
	return addonsInfo
}

func isEqualAddons(new, old []model.AddonsData) bool {
	if len(old) == 0 {
		return false
	}
	if len(new) != len(old) {
		return false
	}
	return reflect.DeepEqual(getAddonMap(new), getAddonMap(old))
}

func getAddonMap(addonList []model.AddonsData) map[string]model.AddonsData {
	addonMap := map[string]model.AddonsData{}
	for _, item := range addonList {
		addonMap[item.Name] = item
	}
	return addonMap
}
