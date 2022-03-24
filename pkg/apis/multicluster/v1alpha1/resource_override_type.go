package v1alpha1

import (
	"harmonycloud.cn/stellaris/pkg/apis/multicluster/common"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type MultiClusterResourceOverride struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   *MultiClusterResourceOverrideSpec   `json:"spec,omitempty"`
}

type MultiClusterResourceOverrideSpec struct {
	Resources	 	*MultiClusterResourceOverrideResources 	`json:"resources"`
	Clusters 		[]MultiClusterResourceOverrideClusters 	`json:"clusters"`
}

type MultiClusterResourceOverrideClusters struct {
	Names 	[]string `json:"name"`
	Overrides		[]common.JSONPatch 	`json:"overrides"`
}

type MultiClusterResourceOverrideResources struct {
	Names	[]string `json:"name"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type MultiClusterResourceOverrideList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []MultiClusterResourceOverride `json:"items"`
}