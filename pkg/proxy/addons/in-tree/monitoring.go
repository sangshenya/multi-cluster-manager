package inTree

import (
	"context"
	"encoding/json"
	"errors"

	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	proxy_cfg "harmonycloud.cn/stellaris/pkg/proxy/config"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"harmonycloud.cn/stellaris/pkg/model"
)

type MonitoringAddonInfo struct {
	Info       []model.AddonsInfo `json:"info"`
	ConfigInfo *model.ConfigInfo  `json:"configInfo"`
}

type MonitoringAddonsInfoData struct {
	// Time duration Prometheus shall retain data for. Default is '24h' if
	// retentionSize is not set, and must match the regular expression `[0-9]+(ms|s|m|h|d|w|y)`
	// (milliseconds seconds minutes hours days weeks years).
	Retention string `json:"retention,omitempty"`
	// Interval between consecutive scrapes.
	ScrapeInterval string `json:"scrapeInterval,omitempty"`
	// Version of Prometheus to be deployed.
	Version string `json:"version"`
	// QuerySpec defines the query command line flags when starting Prometheus.
	Query *QuerySpec `json:"query,omitempty"`
}

type QuerySpec struct {
	// The delta difference allowed for retrieving metrics during expression evaluations.
	LookbackDelta *string `json:"lookbackDelta,omitempty"`
	// Number of concurrent queries that can be run at once.
	MaxConcurrency *int32 `json:"maxConcurrency,omitempty"`
	// Maximum number of samples a single query can load into memory. Note that queries will fail if they would load more samples than this into memory, so this also limits the number of samples a query can return.
	MaxSamples *int32 `json:"maxSamples,omitempty"`
	// Maximum time a query may take before being aborted.
	Timeout *string `json:"timeout,omitempty"`
}

type monitoringAddons struct{}

func (m *monitoringAddons) Load(ctx context.Context, inTree *model.In) (*model.AddonsData, error) {
	if configIsEmpty(inTree.Configurations) {
		return nil, errors.New("in-tree config is empty")
	}
	podList, err := getPodList(ctx, inTree.Configurations.Selector)
	if err != nil {
		return nil, err
	}

	monitoringAddonInfo := &LoggingAddonInfo{
		Info: podHealthInfo(podList),
	}
	configInfo := monitoringConfigInfo(ctx, *inTree.Configurations.ConfigData)
	monitoringAddonInfo.ConfigInfo = configInfo
	return &model.AddonsData{
		Name: inTree.Name,
		Info: monitoringAddonInfo,
	}, nil
}

func monitoringConfigInfo(ctx context.Context, volumes model.ConfigData) *model.ConfigInfo {
	if volumes.ConfigType != model.Prometheus {
		return nil
	}
	prometheusCRDList := &unstructured.UnstructuredList{}

	prometheusCRDList.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "monitoring.coreos.com",
		Version: "v1",
		Kind:    "PrometheusList",
	})

	listOptions := &client.ListOptions{
		Namespace: volumes.Selector.Namespace,
	}
	if volumes.Selector.Labels != nil {
		listOptions.LabelSelector = labels.SelectorFromSet(volumes.Selector.Labels)
	}
	err := proxy_cfg.ProxyConfig.ControllerClient.List(ctx, prometheusCRDList, listOptions)
	if err != nil {
		return nil
	}
	if len(prometheusCRDList.Items) == 0 {
		return nil
	}
	volumesInfo := &model.ConfigInfo{}
	// parse cmData String
	configModel, err := prometheusConfigData(prometheusCRDList.Items[0])
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

func prometheusConfigData(prometheusCRD unstructured.Unstructured) (*MonitoringAddonsInfoData, error) {
	prometheusSpec, ok := prometheusCRD.Object["spec"]
	if !ok || prometheusSpec == nil {
		return nil, errors.New("can not find prometheus config")
	}
	prometheusSpecData, err := json.Marshal(prometheusSpec)
	if err != nil {
		return nil, err
	}

	info := &MonitoringAddonsInfoData{}
	err = json.Unmarshal(prometheusSpecData, info)
	if err != nil {
		return nil, err
	}
	return info, nil
}
