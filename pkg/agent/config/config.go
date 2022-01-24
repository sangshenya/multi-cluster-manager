package config

import (
	multclusterclient "harmonycloud.cn/stellaris/pkg/client/clientset/versioned"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type DefaultConfig struct {
	Cfg              *Configuration
	AgentClient      *multclusterclient.Clientset
	ControllerClient client.Client
}

var AgentConfig *DefaultConfig

func NewAgentConfig(c *Configuration, agentClient *multclusterclient.Clientset, controllerClient client.Client) {
	AgentConfig = &DefaultConfig{}
	AgentConfig.Cfg = c
	AgentConfig.AgentClient = agentClient
	AgentConfig.ControllerClient = controllerClient
}
