package inTree

import (
	"context"

	logf "sigs.k8s.io/controller-runtime/pkg/log"

	agentconfig "harmonycloud.cn/stellaris/pkg/agent/config"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var addonsRegisterLog = logf.Log.WithName("agent_addon_register")

func PodList(ns, label string) (*v1.PodList, error) {
	podList := &v1.PodList{}
	s, err := labels.Parse(label)
	if err != nil {
		return podList, err
	}
	err = agentconfig.AgentConfig.ControllerClient.List(context.Background(), podList, &client.ListOptions{
		LabelSelector: s,
		Namespace:     ns,
	})
	return podList, err
}
