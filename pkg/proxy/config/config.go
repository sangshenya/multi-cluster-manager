package config

import (
	multclusterclient "harmonycloud.cn/stellaris/pkg/client/clientset/versioned"
	multiclusterv1alpha1 "harmonycloud.cn/stellaris/pkg/client/clientset/versioned/typed/multicluster/v1alpha1"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type DefaultConfig struct {
	Cfg              *Configuration
	ProxyClient      *multclusterclient.Clientset
	ControllerClient client.Client
	KubeConfig       *rest.Config
}

var ProxyConfig *DefaultConfig

func NewProxyConfig(c *Configuration, agentClient *multclusterclient.Clientset, controllerClient client.Client, kubeConfig *rest.Config) {
	ProxyConfig = &DefaultConfig{}
	ProxyConfig.Cfg = c
	ProxyConfig.ProxyClient = agentClient
	ProxyConfig.ControllerClient = controllerClient
	ProxyConfig.KubeConfig = kubeConfig
}

func (a *DefaultConfig) ClientV1alpha1() multiclusterv1alpha1.MulticlusterV1alpha1Interface {
	return a.ProxyClient.MulticlusterV1alpha1()
}
