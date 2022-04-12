package v1alpha1

import (
	"harmonycloud.cn/stellaris/pkg/apis/multicluster/common"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:scope="Cluster"
// +kubebuilder:subresource:status
type Cluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec represents the desired behavior.
	Spec ClusterSpec `json:"spec,omitempty"`
	// Status represents the most recently observed status of the Cluster.
	Status ClusterStatus `json:"status,omitempty"`
}

// ClusterSpec represents the expectation of Cluster
type ClusterSpec struct {
	// Addons is the set of cluster addon
	Addons []ClusterAddon `json:"addons,omitempty"`
}

// ClusterStatus represents the overall status of cluster as well as the cluster addon status
type ClusterStatus struct {
	Addons                        []ClusterAddonStatus `json:"addons,omitempty"`
	Conditions                    []common.Condition   `json:"conditions,omitempty"`
	LastReceiveHeartBeatTimestamp metav1.Time          `json:"lastReceiveHeartBeatTimestamp,omitempty"`
	LastUpdateTimestamp           metav1.Time          `json:"lastUpdateTimestamp,omitempty"`
	Healthy                       bool                 `json:"healthy,omitempty"`
	Status                        ClusterStatusType    `json:"status,omitempty"`
}

// ClusterAddonType is the set of cluster addon that can be used in a cluster.
type ClusterAddonType string

// These are valid cluster addon operators.
const (
	InTreeType  ClusterAddonType = "in-tree"
	OutTreeType ClusterAddonType = "out-tree"
)

// ClusterAddon represents the identifier of cluster addon
type ClusterAddon struct {
	// Type of cluster addon
	Type ClusterAddonType `json:"type"`
	// Name of cluster addon
	Name string `json:"name"`
	// Configuration of cluster addon
	// +kubebuilder:pruning:PreserveUnknownFields
	Configuration *runtime.RawExtension `json:"configuration,omitempty"`
}

// ClusterAddonStatus represents the cluster addon status
type ClusterAddonStatus struct {
	// Name of cluster addon
	Name string `json:"name,omitempty"`
	// +kubebuilder:pruning:PreserveUnknownFields
	Info *runtime.RawExtension `json:"info,omitempty"`
}

// ClusterStatusType is the set of cluster status that can be used in a cluster.
type ClusterStatusType string

// These are valid cluster status operators.
const (
	OnlineStatus       ClusterStatusType = "online"
	OfflineStatus      ClusterStatusType = "offline"
	InitializingStatus ClusterStatusType = "initializing"
)

// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterList is a collection of Cluster
type ClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	// Items holds a list of Cluster.
	Items []Cluster `json:"items"`
}
