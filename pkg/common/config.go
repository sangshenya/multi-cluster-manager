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

// TODO clusterName change to clusterNamespace
func ClusterNamespace(clusterName string) string {
	return clusterName
}

func ClusterName(clusterNamespace string) string {
	return clusterNamespace
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

// TODO should determine the cluster role
func IsControlPlane() bool {
	return true
}
