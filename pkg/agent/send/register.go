package send

import (
	"errors"
	"fmt"

	"harmonycloud.cn/stellaris/pkg/agent/addons"
	agentconfig "harmonycloud.cn/stellaris/pkg/agent/config"
	agentStream "harmonycloud.cn/stellaris/pkg/agent/stream"
	"harmonycloud.cn/stellaris/pkg/model"
	"harmonycloud.cn/stellaris/pkg/util/agent"
	"harmonycloud.cn/stellaris/pkg/util/common"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var registerLog = logf.Log.WithName("agent_send_register")

func Register() error {
	registerLog.Info(fmt.Sprintf("start register cluster(%s)", agentconfig.AgentConfig.Cfg.ClusterName))
	stream := agentStream.GetConnection()
	if stream == nil {
		err := errors.New("get stream failed")
		registerLog.Error(err, "register")
		return err
	}
	addonInfo := &model.RegisterRequest{}
	if agentconfig.AgentConfig.Cfg.AddonPath != "" {
		addonConfig, err := agent.GetAddonConfig(agentconfig.AgentConfig.Cfg.AddonPath)
		if err != nil {
			registerLog.Error(err, "get addons config failed")
			return err
		}
		addonsList := addons.LoadAddon(addonConfig)
		addonInfo.Addons = addonsList
	}

	request, err := common.GenerateRequest(model.Register.String(), addonInfo, agentconfig.AgentConfig.Cfg.ClusterName)
	if err != nil {
		registerLog.Error(err, "create request failed")
		return err
	}
	if err := stream.Send(request); err != nil {
		registerLog.Error(err, "send request failed")
		return err
	}

	return nil
}
