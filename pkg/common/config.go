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
)
const (
	ClusterWorkspacePrefix = "stellaris-harmonycloud-cn-"
)

const (
	NamespaceMappingLabel = "stellaris.harmonycloud.cn.namespacemapping/"
)

const (
	ClusterControllerFinalizer = "stellaris/cluster-controller"
)

const (
	NamespaceMappingControllerFinalizer = "stellaris/namespace-mapping-controller"
)

func ClusterNamespace(clusterName string) string {
	return ClusterWorkspacePrefix + clusterName
}

func ClusterName(clusterNamespace string) string {
	if strings.Contains(clusterNamespace, ClusterWorkspacePrefix) {
		return strings.Replace(clusterNamespace, ClusterWorkspacePrefix, "", -1)
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
	return labels.Parse(MultiClusterResourceLabelName + "." + multiClusterResourceName + "=1")
}
