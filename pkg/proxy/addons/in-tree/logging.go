package inTree

import (
	"context"
	"errors"

	"gopkg.in/yaml.v2"

	"harmonycloud.cn/stellaris/pkg/model"
)

type LoggingAddonInfo struct {
	Info       []model.AddonsInfo `json:"info"`
	ConfigInfo *model.ConfigInfo  `json:"configInfo"`
}

type LoggingAddonsInfoData struct {
	ElasticUsername string `json:"elasticUsername"`
	ElasticPassword string `json:"elasticPassword"`
	ElasticURL      string `json:"elasticURL"`
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

	configInfo := loggingVolumesInfo(ctx, *inTree.Configurations.ConfigData)
	loggingAddonInfo.ConfigInfo = configInfo
	return &model.AddonsData{
		Name: inTree.Name,
		Info: loggingAddonInfo,
	}, nil
}

func loggingVolumesInfo(ctx context.Context, volumes model.ConfigData) *model.ConfigInfo {
	if volumes.ConfigType != model.Env {
		return nil
	}
	cmList, err := getConfigMapList(ctx, *volumes.Selector)
	if err != nil {
		return nil
	}
	var cmDataString string
	for _, cm := range cmList {
		cmDataString, err = getConfigMapData(cm, volumes.DataKey)
		if err != nil {
			continue
		}
		if len(cmDataString) > 0 {
			break
		}
	}
	volumesInfo := &model.ConfigInfo{}
	// parse cmData String
	configModel, err := LoggingConfigData(cmDataString)
	if err != nil {
		volumesInfo = &model.ConfigInfo{
			Message: err.Error(),
		}
	} else {
		volumesInfo = &model.ConfigInfo{
			Data:    configModel,
			Message: "success",
		}
	}
	return volumesInfo
}

func LoggingConfigData(esConfigString string) (*LoggingAddonsInfoData, error) {
	var m map[string]string
	err := yaml.Unmarshal([]byte(esConfigString), &m)
	if err != nil {
		return nil, err
	}
	var info *LoggingAddonsInfoData
	if username, ok := m["elasticsearch.username"]; ok {
		info.ElasticUsername = username
	}
	if url, ok := m["elasticsearch.url"]; ok {
		info.ElasticURL = url
	}
	if password, ok := m["elasticsearch.password"]; ok {
		info.ElasticPassword = password
	}
	return info, nil
}
