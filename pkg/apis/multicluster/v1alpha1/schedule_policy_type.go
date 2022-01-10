package v1alpha1

import (
	"harmonycloud.cn/stellaris/pkg/apis/multicluster/common"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
type MultiClusterResourceSchedulePolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MultiClusterResourceSchedulePolicySpec   `json:"spec,omitempty"`
	Status MultiClusterResourceSchedulePolicyStatus `json:"status,omitempty"`
}

type ClusterSourceType string

const (
	ClusterSourceTypeAssign     ClusterSourceType = "assign"
	ClusterSourceTypeClusterset ClusterSourceType = "clusterset"
)

type ScheduleModeType string

const (
	ScheduleModeTypeWeighted   ScheduleModeType = "Weighted"
	ScheduleModeTypeDuplicated ScheduleModeType = "Duplicated"
)

type MultiClusterResourceSchedulePolicySpec struct {
	Resources      []SchedulePolicyResource `json:"resources,omitempty"`
	ClusterSource  ClusterSourceType        `json:"clusterSource,omitempty"`
	Clusterset     string                   `json:"clusterset,omitempty"`
	Replicas       int                      `json:"replicas"`
	ScheduleMode   ScheduleModeType         `json:"scheduleMode,omitempty"`
	Reschedule     bool                     `json:"reschedule,omitempty"`
	Policy         []SchedulePolicy         `json:"policy,omitempty"`
	FailoverPolicy []ScheduleFailoverPolicy `json:"failoverPolicy,omitempty"`
}

type SchedulePolicyResource struct {
	Name string `json:"name"`
}

type SchedulePolicy struct {
	Name   string `json:"name"`
	Weight int    `json:"weight,omitempty"`
	Min    int    `json:"min,omitempty"`
	Max    int    `json:"max,omitempty"`
}

type ScheduleFailoverPolicy struct {
	Name string             `json:"name,omitempty"`
	Type common.ClusterType `json:"type,omitempty"`
}

type MultiClusterResourceSchedulePolicyStatus struct {
	Schedule ScheduleStatus `json:"schedule,omitempty"`
}

type ScheduleStatus struct {
	Status           bool         `json:"status,omitempty"`
	Message          string       `json:"message,omitempty"`
	LastScheduleTime *metav1.Time `json:"lastScheduleTime,omitempty"`
	LastModifyTime   *metav1.Time `json:"lastModifyTime,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type MultiClusterResourceSchedulePolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []MultiClusterResourceSchedulePolicy `json:"items"`
}
