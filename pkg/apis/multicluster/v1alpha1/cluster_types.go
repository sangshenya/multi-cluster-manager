package v1alpha1

import (
	"harmonycloud.cn/stellaris/pkg/apis/multicluster/common"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type Cluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterSpec   `json:"spec,omitempty"`
	Status ClusterStatus `json:"status,omitempty"`
}

type ClusterSpec struct {
	Addons []ClusterAddons `json:"addons,omitempty"`
}

type ClusterStatus struct {
	Conditions                    []common.Condition `json:"conditions,omitempty"`
	LastReceiveHeartBeatTimestamp metav1.Time        `json:"lastReceiveHeartBeatTimestamp,omitempty"`
	LastUpdateTimestamp           metav1.Time        `json:"lastUpdateTimestamp,omitempty"`
	Healthy                       bool               `json:"healthy,omitempty"`
	Status                        ClusterStatusType  `json:"status,omitempty"`
}

type ClusterAddons struct {
	Name string                `json:"name"`
	Info *runtime.RawExtension `json:"info"`
}

type ClusterStatusType string

const (
	OnlineStatus  ClusterStatusType = "online"
	OfflineStatus ClusterStatusType = "offline"
)

// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Cluster `json:"items"`
}
