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
	// Spec represents the desired behavior.
	Spec ClusterSetSpec `json:"spec,omitempty"`
}

// ClusterSetSpec represents the expectation of ClusterSet
type ClusterSetSpec struct {
	// Selector A label query over a set of ClusterSet.
	// If Clusters is not empty, Selector will be ignored.
	Selector ClusterSetSelector `json:"clusterSelector,omitempty"`
	// Clusters is the set of the target Cluster.
	Clusters []ClusterSetTarget `json:"clusters,omitempty"`
	// TODO this field should be enum
	Policy string `json:"policy,omitempty"`
}

// ClusterSetTarget the description of Cluster
type ClusterSetTarget struct {
	// Name of cluster
	Name string `json:"name,omitempty"`
	// Role of cluster
	Role string `json:"role,omitempty"`
}

// ClusterSetSelector is a field filter.
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
