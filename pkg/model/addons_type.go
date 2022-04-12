package model

import "harmonycloud.cn/stellaris/pkg/apis/multicluster/v1alpha1"

type AddonsCmConfig struct {
	Addons []v1alpha1.ClusterAddon `json:"addons,omitempty"`
}

type AddonsConfig struct {
	Addons *Addons `json:"addons"`
}

type AddonType string

const (
	AddonTypeOut AddonType = "outTree"
	AddonTypeIn  AddonType = "inTree"
)

type Addons struct {
	InTree  []In  `json:"inTree"`
	OutTree []Out `json:"outTree"`
}

type In struct {
	Name           string        `json:"name"`
	Configurations *InTreeConfig `json:"configurations"`
}

type Out struct {
	Name           string         `json:"name"`
	Configurations *OutTreeConfig `json:"configurations"`
}

type ConfigType string

const (
	Env        ConfigType = "env"
	ConfigMap  ConfigType = "ConfigMap"
	Prometheus ConfigType = "prometheus"
)

type InTreeConfig struct {
	Selector   []Selector  `json:"selector,omitempty"`
	Static     []Static    `json:"static,omitempty"`
	ConfigData *ConfigData `json:"configData,omitempty"`
}

type ConfigData struct {
	ConfigType ConfigType             `json:"configType"`
	Selector   *Selector              `json:"selector"`
	KeyList    []string               `json:"keyList,omitempty"`
	Update     map[string]interface{} `json:"update,omitempty"`
}

type Selector struct {
	Namespace string            `json:"namespace"`
	Labels    map[string]string `json:"labels,omitempty"`
	Include   string            `json:"include,omitempty"`
}

type Static struct {
	Endpoint string `json:"endpoint"`
}

type OutTreeConfig struct {
	HTTP *HTTP `json:"http"`
}

type HTTP struct {
	URL []string `json:"url"`
}

// AddonsData addons response data
type AddonsData struct {
	Name string      `json:"name"`
	Info interface{} `json:"info"`
}

type AddonInfoSourceType string

const (
	AddonInfoSourcePod    AddonInfoSourceType = "Pod"
	AddonInfoSourceStatic AddonInfoSourceType = "Static"
)

type AddonStatusType string

const (
	AddonStatusTypeReady    AddonStatusType = "Ready"
	AddonStatusTypeNotReady AddonStatusType = "NotReady"
)

type AddonsInfo struct {
	Type      AddonInfoSourceType `json:"type"`
	Address   string              `json:"address"`
	TargetRef *TargetResource     `json:"targetRef,omitempty"`
	Data      interface{}         `json:"data,omitempty"`
	Status    AddonStatusType     `json:"status"`
}

type TargetResource struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

type ConfigInfo struct {
	Data    interface{} `json:"data,omitempty"`
	Message string      `json:"message"`
}

type CommonAddonInfo struct {
	Info []AddonsInfo `json:"infos"`
}
