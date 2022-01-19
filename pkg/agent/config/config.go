package config

import (
	multclusterclient "harmonycloud.cn/stellaris/pkg/client/clientset/versioned"
)

type DefaultConfig struct {
	Cfg         *Configuration
	AgentClient *multclusterclient.Clientset
}

var AgentConfig *DefaultConfig

func NewAgentConfig(c *Configuration, client *multclusterclient.Clientset) {
	AgentConfig = &DefaultConfig{}
	AgentConfig.Cfg = c
	AgentConfig.AgentClient = client
}
