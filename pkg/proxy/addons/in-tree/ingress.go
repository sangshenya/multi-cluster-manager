package inTree

import "harmonycloud.cn/stellaris/pkg/model"

type ingressAddons struct{}

type IngressAddonsData struct {
	PodIP []string `json:"podIP"`
}

func (i *ingressAddons) Load() (*model.PluginsData, error) {
	// TODO get ingress info
	return nil, nil
}
