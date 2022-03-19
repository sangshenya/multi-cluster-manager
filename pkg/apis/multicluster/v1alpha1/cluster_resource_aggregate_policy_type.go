package v1alpha1

import (
	"harmonycloud.cn/stellaris/pkg/apis/multicluster/common"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status

type MultiClusterResourceAggregatePolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MultiClusterResourceAggregatePolicySpec   `json:"spec,omitempty"`
	Status MultiClusterResourceAggregatePolicyStatus `json:"status,omitempty"`
}

type AggregatePolicyType string

const (
	AggregatePolicySameNsMappingName AggregatePolicyType = "sameNamespaceMappingName"
)

type MultiClusterResourceAggregatePolicySpec struct {
	AggregateRules []string                 `json:"aggregateRules"`
	Clusters       *AggregatePolicyClusters `json:"clusters"`
	Policy         AggregatePolicyType      `json:"policy"`
	Limit          *AggregatePolicyLimit    `json:"limit,omitempty"`
}

type AggregatePolicyClusters struct {
	ClusterType common.ClusterType `json:"clusterType"`
	Clusters    []string           `json:"clusters,omitempty"`
	Clusterset  string             `json:"clusterset,omitempty"`
}

type AggregatePolicyLimit struct {
	Requests *AggregatePolicyLimitRule `json:"requests,omitempty"`
	Ignores  *AggregatePolicyLimitRule `json:"ignores,omitempty"`
}

type AggregatePolicyLimitRule struct {
	LabelsMatch *LabelsMatch `json:"labelsMatch,omitempty"`
	Match       []Match      `json:"match,omitempty"`
}

type LabelsMatch struct {
	LabelSelector     *metav1.LabelSelector `json:"labelSelector,omitempty"`
	NamespaceSelector *metav1.LabelSelector `json:"namespaceSelector,omitempty"`
}

type Match struct {
	Namespaces string      `json:"namespaces,omitempty"`
	NameMatch  *MatchScope `json:"nameMatch,omitempty"`
}

type MatchScope struct {
	Include string   `json:"include,omitempty"`
	List    []string `json:"list,omitempty"`
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
