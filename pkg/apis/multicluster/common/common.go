package common

import (
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

// +k8s:deepcopy-gen=true

type JSONPatch struct {
	Op    string `json:"op,omitempty"`
	Value apiextensionsv1.JSON  `json:"value,omitempty"`
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

type ClusterType string

const (
	ClusterTypeClusterSet ClusterType = "clusterset"
	ClusterTypeClusters   ClusterType = "clusters"
)
