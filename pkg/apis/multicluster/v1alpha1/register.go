package v1alpha1

import (
	"reflect"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var SchemeGroupVersion = schema.GroupVersion{
	Group:   "multicluster.harmonycloud.cn",
	Version: "v1alpha1",
}

func Kind(kind string) schema.GroupKind {
	return SchemeGroupVersion.WithKind(kind).GroupKind()
}

func Resource(resource string) schema.GroupResource {
	return SchemeGroupVersion.WithResource(resource).GroupResource()
}

var (
	SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)
	AddToScheme   = SchemeBuilder.AddToScheme
)

// Adds the list of known types to Scheme.
func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(SchemeGroupVersion,
		&Cluster{},
		&ClusterList{},
		&ClusterSet{},
		&ClusterSetList{},
		&NamespaceMapping{},
		&NamespaceMappingList{},
		&ClusterResource{},
		&ClusterResourceList{},
		&ResourceAggregatePolicy{},
		&ResourceAggregatePolicyList{},
		&AggregatedResource{},
		&AggregatedResourceList{},
		&MultiClusterResourceAggregatePolicy{},
		&MultiClusterResourceAggregatePolicyList{},
		&MultiClusterResourceAggregateRule{},
		&MultiClusterResourceAggregateRuleList{},
		&MultiClusterResource{},
		&MultiClusterResourceList{},
		&MultiClusterResourceBinding{},
		&MultiClusterResourceBindingList{},
		&MultiClusterResourceSchedulePolicy{},
		&MultiClusterResourceSchedulePolicyList{},
	)
	metav1.AddToGroupVersion(scheme, SchemeGroupVersion)
	return nil
}

// AggregatedResource type metadata
var (
	AggregatedResourceKind             = reflect.TypeOf(AggregatedResource{}).Name()
	AggregatedResourceGroupVersionKind = SchemeGroupVersion.WithKind(AggregatedResourceKind)
)

// ClusterResource type metadata
var (
	ClusterResourceKind             = reflect.TypeOf(ClusterResource{}).Name()
	ClusterResourceGroupVersionKind = SchemeGroupVersion.WithKind(ClusterResourceKind)
)

// MultiClusterResourceAggregatePolicy type metadata
var (
	MultiClusterResourceAggregatePolicyKind             = reflect.TypeOf(MultiClusterResourceAggregatePolicy{}).Name()
	MultiClusterResourceAggregatePolicyGroupVersionKind = SchemeGroupVersion.WithKind(MultiClusterResourceAggregatePolicyKind)
)

// MultiClusterResourceAggregateRule type metadata
var (
	MultiClusterResourceAggregateRuleKind             = reflect.TypeOf(MultiClusterResourceAggregateRule{}).Name()
	MultiClusterResourceAggregateRuleGroupVersionKind = SchemeGroupVersion.WithKind(MultiClusterResourceAggregateRuleKind)
)

// ClusterSet type metadata
var (
	ClusterSetKind             = reflect.TypeOf(ClusterSet{}).Name()
	ClusterSetGroupVersionKind = SchemeGroupVersion.WithKind(ClusterSetKind)
)

// Cluster type metadata
var (
	ClusterKind             = reflect.TypeOf(Cluster{}).Name()
	ClusterGroupVersionKind = SchemeGroupVersion.WithKind(ClusterKind)
)

// NamespaceMapping type metadata
var (
	NamespaceMappingKind             = reflect.TypeOf(NamespaceMapping{}).Name()
	NamespaceMappingGroupVersionKind = SchemeGroupVersion.WithKind(NamespaceMappingKind)
)

// ResourceAggregatePolicy type metadata
var (
	ResourceAggregatePolicyKind             = reflect.TypeOf(ResourceAggregatePolicy{}).Name()
	ResourceAggregatePolicyGroupVersionKind = SchemeGroupVersion.WithKind(ResourceAggregatePolicyKind)
)

// MultiClusterResourceBinding type metadata
var (
	MultiClusterResourceBindingKind             = reflect.TypeOf(MultiClusterResourceBinding{}).Name()
	MultiClusterResourceBindingGroupVersionKind = SchemeGroupVersion.WithKind(MultiClusterResourceBindingKind)
)

// MultiClusterResource type metadata
var (
	MultiClusterResourceKind             = reflect.TypeOf(MultiClusterResource{}).Name()
	MultiClusterResourceGroupVersionKind = SchemeGroupVersion.WithKind(MultiClusterResourceKind)
)

// MultiClusterResourceSchedulePolicy type metadata
var (
	MultiClusterResourceSchedulePolicyKind             = reflect.TypeOf(MultiClusterResourceSchedulePolicy{}).Name()
	MultiClusterResourceSchedulePolicyGroupVersionKind = SchemeGroupVersion.WithKind(MultiClusterResourceSchedulePolicyKind)
)
