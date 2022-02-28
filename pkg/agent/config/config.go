package config

import (
	multclusterclient "harmonycloud.cn/stellaris/pkg/client/clientset/versioned"
	multiclusterv1alpha1 "harmonycloud.cn/stellaris/pkg/client/clientset/versioned/typed/multicluster/v1alpha1"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type DefaultConfig struct {
	Cfg              *Configuration
	AgentClient      *multclusterclient.Clientset
	ControllerClient client.Client
	KubeConfig       *rest.Config
}

var AgentConfig *DefaultConfig

func NewAgentConfig(c *Configuration, agentClient *multclusterclient.Clientset, controllerClient client.Client, kubeConfig *rest.Config) {
	AgentConfig = &DefaultConfig{}
	AgentConfig.Cfg = c
	AgentConfig.AgentClient = agentClient
	AgentConfig.ControllerClient = controllerClient
	AgentConfig.KubeConfig = kubeConfig
}

func (a *DefaultConfig) ClientV1alpha1() multiclusterv1alpha1.MulticlusterV1alpha1Interface {
	return a.AgentClient.MulticlusterV1alpha1()
}
