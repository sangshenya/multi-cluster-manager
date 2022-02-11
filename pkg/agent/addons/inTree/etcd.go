package inTree

import (
	"errors"

	"github.com/google/uuid"
	"harmonycloud.cn/stellaris/pkg/model"
)

type etcdAddons struct{}

type EtcdAddonsInfo struct {
	PodIP []string
}

func (e *etcdAddons) Load() (*model.PluginsData, error) {
	etcdPodList, err := PodList("kube-system", "component=etcd")
	if err != nil {
		addonsRegisterLog.Error(err, "get apiServer pod list failed")
		return nil, err
	}
	if len(etcdPodList.Items) == 0 {
		err = errors.New("can not find apiServer pod list")
		addonsRegisterLog.Error(err, "")
		return nil, err
	}
	etcdData := &model.PluginsData{
		Uid:  uuid.NewString(),
		Data: nil,
	}

	etcdInfo := &EtcdAddonsInfo{}
	for _, item := range etcdPodList.Items {
		etcdInfo.PodIP = append(etcdInfo.PodIP, item.Status.PodIP)
	}
	etcdData.Data = etcdInfo
	return etcdData, nil
}
