package common

import (
	"errors"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

const (
	// TODO set manager default namespace
	ManagerNamespace = "chenkun"
	// TODO set manager default FinalizerName
	FinalizerName                               = "multicluster.harmonycloud.cn.Finalizer"
	ClusterResourceLabelName                    = "multicluster.harmonycloud.cn.ClusterResource"
	ResourceBindingLabelName                    = "multicluster.harmonycloud.cn.ResourceBinding"
	ResourceGvkLabelName                        = "multicluster.harmonycloud.cn.ResourceGvk"
	MultiClusterResourceLabelName               = "multicluster.harmonycloud.cn.MultiClusterResource"
	MultiClusterResourceSchedulePolicyLabelName = "multicluster.harmonycloud.cn.SchedulePolicy"
)

// TODO clusterName change to clusterNamespace
func ClusterNamespace(clusterName string) string {
	return clusterName
}

func ClusterName(clusterNamespace string) string {
	return clusterNamespace
}

func GvkLabelString(gvk *metav1.GroupVersionKind) string {
	gvkString := gvk.Group + ":" + gvk.Version + ":" + gvk.Kind
	if len(gvk.Group) == 0 {
		gvkString = gvk.Version + ":" + gvk.Kind
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
