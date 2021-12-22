package v1alpha1

import (
	"harmonycloud.cn/stellaris/pkg/apis/multicluster/common"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
type MultiClusterResource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MultiClusterResourceSpec   `json:"spec,omitempty"`
	Status MultiClusterResourceStatus `json:"status,omitempty"`
}

type MultiClusterResourceSpec struct {
	Resource      *runtime.RawExtension    `json:"resource,omitempty"`
	ResourceRef   *metav1.GroupVersionKind `json:"resourceRef,omitempty"`
	ReplicasField string                   `json:"replicasFiled,omitempty"`
	Workload      bool                     `json:"workload,omitempty"`
}

type MultiClusterResourceStatus struct {
	ClusterStatus []common.MultiClusterResourceClusterStatus `json:"clusters,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type MultiClusterResourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []MultiClusterResource `json:"items"`
}
