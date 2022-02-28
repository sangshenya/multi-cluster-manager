package config

import (
	multclusterclient "harmonycloud.cn/stellaris/pkg/client/clientset/versioned"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type DefaultConfig struct {
	Cfg              *Configuration
	ProxyClient      *multclusterclient.Clientset
	ControllerClient client.Client
}

var ProxyConfig *DefaultConfig

func NewProxyConfig(c *Configuration, proxyClient *multclusterclient.Clientset, controllerClient client.Client) {
	ProxyConfig = &DefaultConfig{}
	ProxyConfig.Cfg = c
	ProxyConfig.ProxyClient = proxyClient
	ProxyConfig.ControllerClient = controllerClient
}
