package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +genclient
// +genclient:nonNamespaced
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:scope="Cluster"
type ClusterSet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ClusterSetSpec `json:"spec,omitempty"`
}

type ClusterSetSpec struct {
	Selector ClusterSetSelector `json:"clusterSelector,omitempty"`
	Clusters []ClusterSetTarget `json:"clusters,omitempty"`
	// TODO this field should be enum
	Policy string `json:"policy,omitempty"`
}

type ClusterSetTarget struct {
	Name string `json:"name,omitempty"`
	Role string `json:"role,omitempty"`
}

type ClusterSetSelector struct {
	Labels map[string]string `json:"labels,omitempty"`
}

// +genclient:nonNamespaced
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ClusterSetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []ClusterSet `json:"items"`
}
