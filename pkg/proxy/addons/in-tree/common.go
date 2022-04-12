package inTree

import (
	"context"
	"errors"
	"strings"

	"harmonycloud.cn/stellaris/pkg/model"
	proxy_cfg "harmonycloud.cn/stellaris/pkg/proxy/config"
	"harmonycloud.cn/stellaris/pkg/utils/httprequest"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func checkHealthyWithURL(url string) bool {
	response, err := httprequest.HttpGetWithEmptyHeader(url)
	if err != nil {
		return false
	}
	if response.StatusCode != 200 {
		return false
	}
	return true
}

func getConfigMapList(ctx context.Context, selector model.Selector) ([]v1.ConfigMap, error) {
	var configMaps []v1.ConfigMap

	cmList := &v1.ConfigMapList{}
	listOptions := &client.ListOptions{
		Namespace: selector.Namespace,
	}
	if selector.Labels != nil {
		listOptions.LabelSelector = labels.SelectorFromSet(selector.Labels)
	}
	err := proxy_cfg.ProxyConfig.ControllerClient.List(ctx, cmList, listOptions)
	if err != nil {
		return nil, err
	}
	for _, cm := range cmList.Items {
		if len(selector.Include) > 0 {
			if !strings.Contains(cm.GetName(), selector.Include) {
				continue
			}
		}
		configMaps = append(configMaps, cm)
	}
	return configMaps, nil
}

func getPodList(ctx context.Context, selectors []model.Selector) ([]v1.Pod, error) {
	var pods []v1.Pod
	for _, selector := range selectors {
		podList := &v1.PodList{}
		listOptions := &client.ListOptions{
			Namespace: selector.Namespace,
		}
		if selector.Labels != nil {
			listOptions.LabelSelector = labels.SelectorFromSet(selector.Labels)
		}
		err := proxy_cfg.ProxyConfig.ControllerClient.List(ctx, podList, listOptions)
		if err != nil {
			return nil, err
		}

		for _, pod := range podList.Items {
			if len(selector.Include) > 0 {
				if !strings.Contains(pod.GetName(), selector.Include) {
					continue
				}
			}
			pods = append(pods, pod)
		}
	}
	return pods, nil
}

func configIsEmpty(inTreeConfig *model.InTreeConfig) bool {
	if inTreeConfig == nil {
		return true
	}
	if len(inTreeConfig.Static) == 0 && len(inTreeConfig.Selector) == 0 {
		return true
	}
	return false
}

func getConfigMapData(cm v1.ConfigMap, dataKey string) (string, error) {
	cmData, ok := cm.Data[dataKey]
	if !ok {
		return "", errors.New("can not get cm data")
	}
	return cmData, nil
}
