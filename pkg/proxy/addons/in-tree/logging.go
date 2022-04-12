package inTree

import (
	"context"
	"errors"
	"strings"

	sliceutils "harmonycloud.cn/stellaris/pkg/utils/slice"

	maputil "harmonycloud.cn/stellaris/pkg/utils/map"

	v1 "k8s.io/api/core/v1"

	"harmonycloud.cn/stellaris/pkg/model"
)

type LoggingAddonInfo struct {
	Info       []model.AddonsInfo `json:"info"`
	ConfigInfo *model.ConfigInfo  `json:"configInfo"`
}

type LoggingAddonsInfoData struct {
	ElasticUsername    string `json:"elasticUsername"`
	ElasticPassword    string `json:"elasticPassword"`
	ElasticPort        string `json:"elasticPort"`
	ElasticClusterName string `json:"elasticClusterName"`
}

type loggingAddons struct{}

func (l *loggingAddons) Load(ctx context.Context, inTree *model.In) (*model.AddonsData, error) {
	if configIsEmpty(inTree.Configurations) {
		return nil, errors.New("in-tree config is empty")
	}
	podList, err := getPodList(ctx, inTree.Configurations.Selector)
	if err != nil {
		return nil, err
	}

	loggingAddonInfo := &LoggingAddonInfo{
		Info: podHealthInfo(podList),
	}

	data := &model.AddonsData{
		Name: inTree.Name,
		Info: loggingAddonInfo,
	}

	if inTree.Configurations.ConfigData.ConfigType == model.Env {
		pod := targetConfigPod(ctx, podList, *inTree.Configurations.ConfigData.Selector)
		if pod == nil {
			return data, nil
		}

		configInfo := loggingConfigInfo(ctx, pod, inTree.Configurations.ConfigData.KeyList)
		loggingAddonInfo.ConfigInfo = configInfo
	}
	return data, nil
}

func targetConfigPod(ctx context.Context, podList []v1.Pod, selector model.Selector) *v1.Pod {
	for _, pod := range podList {
		if pod.GetNamespace() != selector.Namespace {
			continue
		}
		if selector.Labels != nil && maputil.ContainsMap(pod.GetLabels(), selector.Labels) {
			return &pod
		}
		if len(selector.Include) > 0 && strings.Contains(pod.GetName(), selector.Include) {
			return &pod
		}
	}
	// get pod
	pods, err := getPodList(ctx, []model.Selector{selector})
	if err != nil && len(pods) > 0 {
		return &pods[0]
	}
	return nil
}

func loggingConfigInfo(ctx context.Context, pod *v1.Pod, keyList []string) *model.ConfigInfo {
	configInfo := &model.ConfigInfo{}
	if len(pod.Spec.Containers) == 0 || len(pod.Spec.Containers[0].Env) == 0 {
		return nil
	}
	infoData := &LoggingAddonsInfoData{}
	var hasEnv bool
	for _, env := range pod.Spec.Containers[0].Env {
		switch env.Name {
		case "ClusterName":
			if sliceutils.ContainsString(keyList, env.Name) {
				infoData.ElasticClusterName = env.Value
				hasEnv = true
			}
		case "ELASTIC_PASSWORD":
			if sliceutils.ContainsString(keyList, env.Name) {
				infoData.ElasticPassword = env.Value
				infoData.ElasticUsername = "elastic"
				hasEnv = true
			}
		case "TCPPort":
			if sliceutils.ContainsString(keyList, env.Name) {
				infoData.ElasticPort = env.Value
				hasEnv = true
			}
		}
	}
	if !hasEnv {
		configInfo.Message = "can not find config data"
		return configInfo
	}
	configInfo.Message = "success"
	return configInfo
}
