package model

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

type VolumesType string

const (
	ConfigMap VolumesType = "ConfigMap"
)

type InTreeConfig struct {
	Selector    *Selector   `json:"selector,omitempty"`
	Static      []Static    `json:"static,omitempty"`
	VolumesType VolumesType `json:"volumesType,omitempty"`
}

type Selector struct {
	Namespace string            `json:"namespace"`
	Labels    map[string]string `json:"labels,omitempty"`
	Include   string            `json:"include,omitempty"`
}

type Static struct {
	Endpoint string `json:"endpoint"`
}

type Out struct {
	Name string `json:"name"`
	Http *Http  `json:"http"`
}

type Http struct {
	Url string `json:"url"`
}

// addons response data
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
	Status    AddonStatusType     `json:"status"`
}

type TargetResource struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

type VolumesInfo struct {
	Data    interface{} `json:"data,omitempty"`
	Message string      `json:"message"`
}
