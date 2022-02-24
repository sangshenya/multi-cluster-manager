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

	Spec   ClusterSpec   `json:"spec,omitempty"`
	Status ClusterStatus `json:"status,omitempty"`
}

type ClusterSpec struct {
	ApiServer string           `json:"apiserver"`
	SecretRef ClusterSecretRef `json:"secretRef"`
	Addons    []ClusterAddon   `json:"addons,omitempty"`
	// +kubebuilder:pruning:PreserveUnknownFields
	Configuration *runtime.RawExtension `json:"configuration,omitempty"`
}

type ClusterStatus struct {
	Addons                        []ClusterAddonStatus `json:"addons,omitempty"`
	Conditions                    []common.Condition   `json:"conditions,omitempty"`
	LastReceiveHeartBeatTimestamp metav1.Time          `json:"lastReceiveHeartBeatTimestamp,omitempty"`
	LastUpdateTimestamp           metav1.Time          `json:"lastUpdateTimestamp,omitempty"`
	Healthy                       bool                 `json:"healthy,omitempty"`
	Status                        ClusterStatusType    `json:"status,omitempty"`
}

const (
	InTreeType  ClusterAddonType = "in-tree"
	OutTreeType ClusterAddonType = "out-tree"
)

type ClusterAddonType string

type ClusterAddon struct {
	Type ClusterAddonType `json:"type"`
	Name string           `json:"name"`
	Url  string           `json:"url"`
	// +kubebuilder:pruning:PreserveUnknownFields
	Configuration *runtime.RawExtension `json:"configuration,omitempty"`
}

type SecretType string

const (
	KubeConfigType SecretType = "kubeconfig"
	TokenType      SecretType = "token"
)

type ClusterSecretRef struct {
	Type      SecretType `json:"type"`
	Name      string     `json:"name"`
	Namespace string     `json:"namespace"`
	Field     string     `json:"field"`
}

type ClusterAddonStatus struct {
	Name string                `json:"name,omitempty"`
	Info *runtime.RawExtension `json:"info,omitempty"`
}

type ClusterStatusType string

const (
	OnlineStatus       ClusterStatusType = "online"
	OfflineStatus      ClusterStatusType = "offline"
	InitializingStatus ClusterStatusType = "initializing"
)

// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Cluster `json:"items"`
}
