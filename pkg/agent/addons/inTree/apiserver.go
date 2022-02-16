package inTree

import (
	"github.com/google/uuid"
	"harmonycloud.cn/stellaris/pkg/model"
)

type apiServerAddons struct{}

type ApiServerAddonsData struct {
	PodIP []string
}

func (a *apiServerAddons) Load() (*model.PluginsData, error) {
	apiServerPodList, err := PodList("kube-system", "component=kube-apiserver")
	if err != nil {
		addonsRegisterLog.Error(err, "get apiServer pod list failed")
		return nil, err
	}
	if len(apiServerPodList.Items) == 0 {
		addonsRegisterLog.Error(err, "can not find apiServer pod list")
		return nil, err
	}
	apiServerData := &model.PluginsData{
		Uid:  uuid.NewString(),
		Data: nil,
	}

	apiServerInfo := &ApiServerAddonsData{}
	for _, item := range apiServerPodList.Items {
		apiServerInfo.PodIP = append(apiServerInfo.PodIP, item.Status.PodIP)
	}

	apiServerData.Data = apiServerInfo
	return apiServerData, nil
}
