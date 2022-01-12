package addons

import (
	"reflect"
	"time"

	"harmonycloud.cn/stellaris/pkg/agent/condition"

	"github.com/sirupsen/logrus"
	"harmonycloud.cn/stellaris/config"
	agentconfig "harmonycloud.cn/stellaris/pkg/agent/config"
	multclusterclient "harmonycloud.cn/stellaris/pkg/client/clientset/versioned"
	clusterHealth "harmonycloud.cn/stellaris/pkg/common/cluster_health"
	"harmonycloud.cn/stellaris/pkg/model"
	"harmonycloud.cn/stellaris/pkg/util/common"
)

const (
	forceSynchronization = 30
	timeOut              = 10
	HeartbeatMessage     = "ok"
)

func Load(cfg *model.PluginsConfig) (*model.RegisterRequest, *model.AddonsChannel) {
	var channels []chan *model.Addon
	inTreeLen := len(cfg.Plugins.InTree)
	outTreeLen := len(cfg.Plugins.OutTree)
	if inTreeLen <= 0 && outTreeLen <= 0 {
		return &model.RegisterRequest{}, &model.AddonsChannel{}
	}
	if inTreeLen > 0 {
		inTreePlugins := cfg.Plugins.InTree
		for _, name := range inTreePlugins {
			// make channel
			inTreeCh := make(chan *model.Addon)
			channels = append(channels, inTreeCh)

			go runInTreePlugins(name.Name, inTreeCh)
		}
	}
	if outTreeLen > 0 {
		outTreePlugins := cfg.Plugins.OutTree
		for _, name := range outTreePlugins {
			outTreeCh := make(chan *model.Addon)
			channels = append(channels, outTreeCh)

			go runOutTreePlugins(name.Name, outTreeCh)
		}
	}
	addonsChannel := model.AddonsChannel{Channels: channels}
	addonsInfo := getAddonsInfo(addonsChannel)

	result := &model.RegisterRequest{Addons: addonsInfo}
	return result, &addonsChannel
}

func runInTreePlugins(name string, ch chan *model.Addon) {

	res := model.Addon{}
	// TODO RUN PLUGIN
	for {
		// return information
		ch <- &res
	}

}

func runOutTreePlugins(name string, ch chan *model.Addon) {
	res := model.Addon{}
	// TODO RUN PLUGIN
	for {
		// return information
		ch <- &res

	}
}

func startMonitor(name string, ch chan *model.Condition) {
	// TODO

}

func getAddonsInfo(channels model.AddonsChannel) []model.Addon {
	var addons []model.Addon

	for _, channel := range channels.Channels {
		addon := <-channel
		addons = append(addons, *addon)
	}

	return addons

}

func Heartbeat(channel *model.AddonsChannel, stream config.Channel_EstablishClient, cfg *agentconfig.Configuration, agentClient *multclusterclient.Clientset) error {
	var heartbeatWithChange model.HeartbeatWithChangeRequest
	lastHeartbeatTime := time.Now()
	lastHeartbeat := &model.HeartbeatWithChangeRequest{}
	firstTime := true
	for {
		sendFlag := false

		var addonsInfo []model.Addon
		// if plugins are specified
		if len(channel.Channels) > 0 {
			// get info
			addonsInfo = getAddonsInfo(*channel)
			// if not the first time,compare
			if !firstTime {
				for i, addon := range addonsInfo {
					if !reflect.DeepEqual(lastHeartbeat.Addons[i].Properties, addon.Properties) {
						sendFlag = true
					}
				}
			} else {
				firstTime = false
				sendFlag = true
			}
		}
		// get condition
		conditions := condition.GetAgentCondition()
		// CHECK HEALTH
		_, healthy := clusterHealth.GetClusterHealthStatus(agentClient)
		// send
		if (sendFlag) || ((!sendFlag) && ((time.Now().Sub(lastHeartbeatTime)) > forceSynchronization*time.Second)) {
			if len(channel.Channels) > 0 {
				heartbeatWithChange = model.HeartbeatWithChangeRequest{Healthy: healthy, Addons: addonsInfo, Conditions: conditions}
			} else {
				heartbeatWithChange = model.HeartbeatWithChangeRequest{Healthy: healthy}
			}
			lastHeartbeat = &heartbeatWithChange
			request, err := common.GenerateRequest("HeartbeatWithChange", heartbeatWithChange, cfg.ClusterName)
			if err != nil {
				logrus.Error(err)
				continue
			}
			if err := stream.Send(request); err != nil {
				logrus.Error(err)
				continue
			}
			lastHeartbeatTime = time.Now()
			// TODO After Receive Response
			time.Sleep(cfg.HeartbeatPeriod)
		} else {
			request, err := common.GenerateRequest("Heartbeat", HeartbeatMessage, cfg.ClusterName)
			if err != nil {
				logrus.Error(err)
				continue
			}
			if err := stream.Send(request); err != nil {
				logrus.Error(err)
				continue
			}
			lastHeartbeatTime = time.Now()
			time.Sleep(cfg.HeartbeatPeriod)
		}
	}
}
