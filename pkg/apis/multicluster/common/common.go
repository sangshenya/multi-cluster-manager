package common

// +k8s:deepcopy-gen=true

type JSONPatch struct {
	Op    string `json:"op,omitempty"`
	Value string `json:"value,omitempty"`
	Path  string `json:"path,omitempty"`
}

// +k8s:deepcopy-gen=true

type MultiClusterResourceClusterStatus struct {
	Name                      string                    `json:"name,omitempty"`
	Resource                  string                    `json:"resource,omitempty"`
	ObservedReceiveGeneration int64                     `json:"observedReceiveGeneration,omitempty"`
	Phase                     MultiClusterResourcePhase `json:"phase,omitempty"`
	Message                   string                    `json:"message,omitempty"`
	Binding                   string                    `json:"binding,omitempty"`
}

type MultiClusterResourcePhase string

const (
	Creating    MultiClusterResourcePhase = "Creating"
	Complete    MultiClusterResourcePhase = "Complete"
	Terminating MultiClusterResourcePhase = "Terminating"
	Unknown     MultiClusterResourcePhase = "Unknown"
)
