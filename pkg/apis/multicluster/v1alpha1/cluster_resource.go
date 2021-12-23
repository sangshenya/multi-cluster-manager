package v1alpha1

import (
	"harmonycloud.cn/stellaris/pkg/apis/multicluster/common"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status

type ClusterResource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterResourceSpec   `json:"spec,omitempty"`
	Status ClusterResourceStatus `json:"status,omitempty"`
}

type ClusterResourceSpec struct {
	// +kubebuilder:pruning:PreserveUnknownFields
	Resource *runtime.RawExtension `json:"resource,omitempty"`
}

type ClusterResourceStatus struct {
	ObservedReceiveGeneration int64                            `json:"observedReceiveGeneration,omitempty"`
	Phase                     common.MultiClusterResourcePhase `json:"phase,omitempty"`
	Message                   string                           `json:"message,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ClusterResourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []ClusterResource `json:"items"`
}
