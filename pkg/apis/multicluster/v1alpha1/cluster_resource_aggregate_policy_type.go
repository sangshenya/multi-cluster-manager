package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status

type MultiClusterResourceAggregatePolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MultiClusterResourceAggregatePolicySpec   `json:"spec,omitempty"`
	Status MultiClusterResourceAggregatePolicyStatus `json:"status,omitempty"`
}

type AggregatePolicy string

const (
	AggregatePolicySameNsMappingName AggregatePolicy = "sameNamespaceMappingName"
)

type MultiClusterResourceAggregatePolicySpec struct {
	AggregateRules []string                                     `json:"aggregateRules"`
	Clusters       *MultiClusterResourceAggregatePolicyClusters `json:"clusters"`
	Policy         AggregatePolicy                              `json:"policy"`
	Limit          *MultiClusterResourceAggregatePolicyLimit    `json:"limit,omitempty"`
}

type AggregatePolicyClusterType string

const (
	AggregatePolicyClusterTypeClusterSet AggregatePolicyClusterType = "clusterset"
	AggregatePolicyClusterTypeClusters   AggregatePolicyClusterType = "clusters"
)

type MultiClusterResourceAggregatePolicyClusters struct {
	Type       AggregatePolicyClusterType `json:"type"`
	Clusters   []string                   `json:"clusters,omitempty"`
	Clusterset string                     `json:"clusterset,omitempty"`
}

type MultiClusterResourceAggregatePolicyLimit struct {
	Requests []MultiClusterResourceAggregatePolicyLimitRule `json:"requests,omitempty"`
	Ignores  []MultiClusterResourceAggregatePolicyLimitRule `json:"ignores,omitempty"`
}

type MultiClusterResourceAggregatePolicyLimitRule struct {
	Namespaces string                                            `json:"namespaces"`
	NameMatch  MultiClusterResourceAggregatePolicyLimitRuleMatch `json:"nameMatch,omitempty"`
}

type MultiClusterResourceAggregatePolicyLimitRuleMatch struct {
	Regexp string   `json:"regexp,omitempty"`
	List   []string `json:"list,omitempty"`
}

type AggregatePolicyStatus string

const (
	AggregatePolicyStatusNormal     AggregatePolicyStatus = "Normal"
	AggregatePolicyStatusRuleRepeat AggregatePolicyStatus = "RuleRepeat"
)

type MultiClusterResourceAggregatePolicyStatus struct {
	Status  AggregatePolicyStatus `json:"status,omitempty"`
	Message string                `json:"message,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type MultiClusterResourceAggregatePolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MultiClusterResourceAggregatePolicy `json:"items"`
}
