package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status

type AggregatedResource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Clusters *AggregatedResourceClusters `json:"clusters,omitempty"`
	// +kubebuilder:pruning:PreserveUnknownFields
	Aggregation runtime.RawExtension     `json:"aggregation,omitempty"`
	Status      AggregatedResourceStatus `json:"status"`
}

type AggregatedResourceClusters struct {
	Name         string `json:"name"`
	ResourceName string `json:"resourceName"`
	// +kubebuilder:pruning:PreserveUnknownFields
	Result runtime.RawExtension `json:"result,omitempty"`
}

type AggregatedResourceStatus struct {
	Clusters []AggregatedResourceStatusClusters `json:"clusters,omitempty"`
}

type AggregatedResourceStatusClusterStatus string

const (
	// TODO status should be more abundant
	ClusterStatusNormal AggregatedResourceStatusClusterStatus = "normal"
	ClusterStatusError  AggregatedResourceStatusClusterStatus = "error"
)

type AggregatedResourceStatusClusters struct {
	Name         string                                `json:"name"`
	ResourceName string                                `json:"resourceName"`
	UpdateTime   *metav1.Time                          `json:"updateTime"`
	Status       AggregatedResourceStatusClusterStatus `json:"status"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type AggregatedResourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AggregatedResource `json:"items"`
}
