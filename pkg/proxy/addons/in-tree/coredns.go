package inTree

import (
	"bytes"
	"context"
	"errors"
	"strconv"

	proxy_cfg "harmonycloud.cn/stellaris/pkg/proxy/config"
	"k8s.io/apimachinery/pkg/types"

	v1 "k8s.io/api/core/v1"

	"harmonycloud.cn/stellaris/pkg/model"

	"github.com/coredns/caddy"
	"github.com/coredns/caddy/caddyfile"
)

type CoreDNSPluginConfigModel struct {
	EnableErrorLogging bool       `json:"enableErrorLogging"`
	CacheTime          int        `json:"cacheTime"`
	Hosts              []DNSModel `json:"hosts,omitempty"`
	Forward            []DNSModel `json:"forward,omitempty"`
}

type DNSModel struct {
	Domain     string   `json:"domain"`
	Resolution []string `json:"resolution"`
}

type CoreDNSAddonInfo struct {
	Info        []model.AddonsInfo `json:"info"`
	VolumesInfo *model.VolumesInfo `json:"volumesInfo"`
}

type coreDNSAddons struct{}

func (c *coreDNSAddons) Load(ctx context.Context, inTree *model.In) (*model.AddonsData, error) {
	if configIsEmpty(inTree.Configurations) {
		return nil, errors.New("in-tree config is empty")
	}

	podList, err := getPodList(ctx, inTree.Configurations.Selector)
	if err != nil {
		return nil, err
	}

	coreDNSAddonInfo := &CoreDNSAddonInfo{
		Info: podHealthInfo(podList),
	}

	// coreDNS configmap
	if inTree.Configurations.VolumesType == model.ConfigMap {
		volumesInfo := getVolumesInfoWithConfigMap(ctx, podList)
		coreDNSAddonInfo.VolumesInfo = volumesInfo
	}
	return &model.AddonsData{
		Name: inTree.Name,
		Info: coreDNSAddonInfo,
	}, nil
}

func getVolumesInfoWithConfigMap(ctx context.Context, podList []v1.Pod) *model.VolumesInfo {
	var cmDataString string
	var err error
	volumesInfo := &model.VolumesInfo{}
	for _, pod := range podList {
		cmDataString, err = getConfigMapFromPod(ctx, pod)
		if err != nil {
			continue
		}
		if len(cmDataString) > 0 {
			break
		}
	}
	if len(cmDataString) == 0 {
		volumesInfo.Message = "can not find configMap data"
	}
	// parse cmData String
	configModel, err := CoreDNSConfig(cmDataString)
	if err != nil {
		volumesInfo = &model.VolumesInfo{
			Message: err.Error(),
		}
	} else {
		volumesInfo = &model.VolumesInfo{
			Data:    configModel,
			Message: "success",
		}
	}
	return volumesInfo
}

func getConfigMapFromPod(ctx context.Context, pod v1.Pod) (string, error) {
	if len(pod.Spec.Volumes) == 0 {
		return "", errors.New("can not find ConfigMap")
	}

	configMapNamespaced := types.NamespacedName{
		Namespace: pod.Namespace,
	}
	var configMapKey string
	for _, item := range pod.Spec.Volumes {
		if item.ConfigMap != nil && len(item.ConfigMap.Name) > 0 {
			configMapNamespaced.Name = item.ConfigMap.Name
			for _, keyPath := range item.ConfigMap.Items {
				if len(keyPath.Key) > 0 {
					configMapKey = keyPath.Key
					break
				}
			}
			break
		}
	}
	if len(configMapKey) == 0 || len(configMapNamespaced.Name) == 0 {
		return "", errors.New("can not find ConfigMap")
	}

	cm := &v1.ConfigMap{}
	err := proxy_cfg.ProxyConfig.ControllerClient.Get(ctx, configMapNamespaced, cm)
	if err != nil {
		return "", err
	}
	cmData, ok := cm.Data[configMapKey]
	if !ok {
		return "", errors.New("can not get cm data")
	}
	return cmData, nil
}

func CoreDNSConfig(coreDNSCfg string) (*CoreDNSPluginConfigModel, error) {
	validDirectives := caddy.ValidDirectives("dns")
	serverBlocks, err := caddyfile.Parse("", bytes.NewReader([]byte(coreDNSCfg)), validDirectives)
	if err != nil {
		return nil, err
	}
	if len(serverBlocks) == 0 {
		return nil, errors.New("can not parse coredns config")
	}
	pluginConfigModel := &CoreDNSPluginConfigModel{}
	for _, serverBlock := range serverBlocks {
		for key, value := range serverBlock.Tokens {
			switch key {
			case "errors":
				pluginConfigModel.EnableErrorLogging = true
			case "cache":
				if len(value) >= 2 && value[0].Text == "cache" {
					cacheTime, err := strconv.Atoi(value[1].Text)
					if err == nil {
						pluginConfigModel.CacheTime = cacheTime
					}
				}
			case "forward":
				dnsModel := DNSModel{}
				if len(value) < 3 {
					continue
				}
				dnsModel.Domain = value[1].Text
				for i := 2; i < len(value); i++ {
					dnsModel.Resolution = append(dnsModel.Resolution, value[i].Text)
				}
				pluginConfigModel.Forward = append(pluginConfigModel.Forward, dnsModel)
			case "hosts":
				dnsModel := DNSModel{}
				if len(value) < 3 {
					continue
				}
				dnsModel.Domain = value[1].Text
				for i := 2; i < len(value); i++ {
					dnsModel.Resolution = append(dnsModel.Resolution, value[i].Text)
				}
				pluginConfigModel.Hosts = append(pluginConfigModel.Hosts, dnsModel)
			}
		}
	}
	return pluginConfigModel, nil
}
