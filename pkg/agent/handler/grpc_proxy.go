package handler

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"harmonycloud.cn/stellaris/pkg/agent/addons"
	agentconfig "harmonycloud.cn/stellaris/pkg/agent/config"
	"harmonycloud.cn/stellaris/pkg/util/agent"
	"harmonycloud.cn/stellaris/pkg/util/common"
)

func Register(cfg *agentconfig.Configuration) error {
	stream, err := agent.Connection(cfg)
	if err != nil {
		return fmt.Errorf("call err: %v", err)
	}
	addonConfig, err := agent.GetAddonConfig(cfg.AddonPath)
	if err != nil {
		return fmt.Errorf("get addons config err: %v", err)
	}
	addonInfo, channel, err := addons.Load(addonConfig)
	if err != nil {
		return err
	}

	request, err := common.GenerateRequest("Register", addonInfo, cfg.ClusterName)
	if err != nil {
		return err
	}
	if err := stream.Send(request); err != nil {
		return fmt.Errorf("stream send to server err: %v", err)
	}

	resp, err := stream.Recv()
	if err != nil {
		return fmt.Errorf("stream get from server err: %v", err)
	}
	logrus.Printf("stream get from server:%v", resp)
	//TODO After Receive Response
	go addons.Heartbeat(channel, stream, cfg)
	return nil
}
