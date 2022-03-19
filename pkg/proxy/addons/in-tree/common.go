package inTree

import (
	"context"
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

func getPodList(ctx context.Context, selector *model.Selector) ([]v1.Pod, error) {
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
	var pods []v1.Pod
	for _, pod := range podList.Items {
		if len(selector.Include) > 0 {
			if !strings.Contains(pod.GetName(), selector.Include) {
				continue
			}
		}
		pods = append(pods, pod)
	}

	return pods, nil
}

func configIsEmpty(inTreeConfig *model.InTreeConfig) bool {
	if inTreeConfig == nil {
		return true
	}
	if len(inTreeConfig.Static) == 0 && inTreeConfig.Selector == nil {
		return true
	}
	return false
}
