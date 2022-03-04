package inTree

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"harmonycloud.cn/stellaris/pkg/model"
	v1 "k8s.io/api/core/v1"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var addonsKubeLog = logf.Log.WithName("proxy_addon_kube")

type kubeAddons struct{}

func (k *kubeAddons) Load(ctx context.Context, inTree *model.In) (*model.AddonsData, error) {
	if configIsEmpty(inTree.Configurations) {
		return nil, errors.New("in-tree config is empty")
	}

	kubeAddonsInfo, err := getAddonsInfoList(ctx, inTree.Configurations)
	if err != nil {
		return nil, err
	}

	return &model.AddonsData{
		Name: inTree.Name,
		Info: kubeAddonsInfo,
	}, nil
}

func getAddonsInfoList(ctx context.Context, cfg *model.InTreeConfig) ([]model.AddonsInfo, error) {
	var kubeAddonsInfo []model.AddonsInfo
	// selector
	healthInfoList, err := getAddonsInfoWithSelector(ctx, cfg.Selector)
	if err != nil {
		err = fmt.Errorf(fmt.Sprintf("get addons info from selector(%v) failed", cfg.Selector), err)
		return nil, err
	}
	kubeAddonsInfo = append(kubeAddonsInfo, healthInfoList...)

	// static
	staticInfoList := staticPodHealthInfo(cfg.Static)
	kubeAddonsInfo = append(kubeAddonsInfo, staticInfoList...)

	return kubeAddonsInfo, nil
}

func getAddonsInfoWithSelector(ctx context.Context, selector *model.Selector) ([]model.AddonsInfo, error) {
	if selector == nil {
		return nil, nil
	}
	if len(selector.Namespace) <= 0 {
		return nil, errors.New("resource namespace can not be empty")
	}
	pods, err := getPodList(ctx, selector)
	if err != nil {
		return nil, err
	}

	return podHealthInfo(pods), nil
}

func staticPodHealthInfo(staticPods []model.Static) []model.AddonsInfo {
	if len(staticPods) <= 0 {
		return nil
	}
	var addonsInfo []model.AddonsInfo
	var mu sync.Mutex
	wg := sync.WaitGroup{}
	for _, staticPod := range staticPods {
		wg.Add(1)
		go func(pod model.Static) {
			healthy := checkHealthyWithURL(pod.Endpoint)
			info := model.AddonsInfo{
				Type:    model.AddonInfoSourcePod,
				Address: pod.Endpoint,
				Status:  model.AddonStatusTypeNotReady,
			}
			if healthy {
				info.Status = model.AddonStatusTypeReady
			}
			mu.Lock()
			addonsInfo = append(addonsInfo, info)
			mu.Unlock()
			wg.Done()
		}(staticPod)
	}
	wg.Wait()
	return addonsInfo
}

func podHealthInfo(pods []v1.Pod) []model.AddonsInfo {
	var addonsInfo []model.AddonsInfo
	for _, pod := range pods {
		info := model.AddonsInfo{
			Type:    model.AddonInfoSourcePod,
			Address: pod.Status.PodIP,
			TargetRef: &model.TargetResource{
				Namespace: pod.GetNamespace(),
				Name:      pod.GetName(),
			},
			Status: model.AddonStatusTypeNotReady,
		}
		if pod.Status.Phase == v1.PodRunning {
			info.Status = model.AddonStatusTypeReady
		}
		addonsInfo = append(addonsInfo, info)
	}
	return addonsInfo
}
