package common

import (
	"errors"

	"cuelang.org/go/pkg/strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

const (
	// TODO set manager default namespace
	ManagerNamespace = "stellaris-system"
	// TODO set manager default FinalizerName
	FinalizerName                               = "stellaris.Finalizer"
	ClusterResourceLabelName                    = "stellaris.ClusterResource"
	ResourceBindingLabelName                    = "stellaris.ResourceBinding"
	ResourceGvkLabelName                        = "stellaris.ResourceGvk"
	MultiClusterResourceLabelName               = "stellaris.MultiClusterResource"
	MultiClusterResourceSchedulePolicyLabelName = "stellaris.SchedulePolicy"
	V1alpha1Apiversion                          = "stellaris/v1alpha1"
	Scheduler                                   = "multiclusterresourceschedulepolicy"
)

const (
	BindingPathNamespace = "/metadata/namespace"
	BindingOpAdd         = "add"
	BindingOpRemove      = "remove"
	BindingOpReplace     = "replace"
	BindingOpMove        = "move"
	BindingOpCopy        = "copy"
	BindingOpTest        = "test"
)
const (
	ClusterNamespaceInControlPlanePrefix = "stellaris-harmonycloud-cn-"
)

const (
	NamespaceMappingLabel = "stellaris.harmonycloud.cn.namespacemapping/"
)

const (
	ResourceAnnotationKey = "stellaris-annotation"
)

func ClusterNamespace(clusterName string) string {
	return ClusterNamespaceInControlPlanePrefix + clusterName
}

func ClusterName(clusterNamespace string) string {
	if strings.Contains(clusterNamespace, ClusterNamespaceInControlPlanePrefix) {
		return strings.Replace(clusterNamespace, ClusterNamespaceInControlPlanePrefix, "", -1)
	}
	return ""
}

func GvkLabelString(gvk *metav1.GroupVersionKind) string {
	gvkString := gvk.Group + "." + gvk.Version + "." + strings.ToLower(gvk.Kind)
	if len(gvk.Group) == 0 {
		gvkString = gvk.Version + "." + strings.ToLower(gvk.Kind)
	}
	return gvkString
}

func GetMultiClusterResourceSelectorForMultiClusterResourceName(multiClusterResourceName string) (labels.Selector, error) {
	if len(multiClusterResourceName) == 0 {
		return nil, errors.New("multiClusterResourceName is empty")
	}
	str := MultiClusterResourceLabelName + "." + multiClusterResourceName + "=1"
	return labels.Parse(str)
}
