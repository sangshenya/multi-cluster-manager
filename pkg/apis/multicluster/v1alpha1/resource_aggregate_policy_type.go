package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status

type ResourceAggregatePolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ResourceAggregatePolicySpec   `json:"spec,omitempty"`
	Status ResourceAggregatePolicyStatus `json:"status,omitempty"`
}

type ResourceAggregatePolicySpec struct {
	ResourceRef *metav1.GroupVersionKind `json:"resourceRef,omitempty"`
	Limit       *AggregatePolicyLimit    `json:"limit,omitempty"`
}

type ResourceAggregatePolicyStatus struct {
	// TODO should define status
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ResourceAggregatePolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ResourceAggregatePolicy `json:"items"`
}
