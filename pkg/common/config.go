package common

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

const (
	ManagerNamespace                            = ""
	FinalizerName                               = ""
	ResourceBindingLabelName                    = "multicluster.harmonycloud.cn.ResourceBinding"
	ResourceGvkLabelName                        = "multicluster.harmonycloud.cn.ResourceGvk"
	MultiClusterResourceLabelName               = "multicluster.harmonycloud.cn.MultiClusterResource"
	MultiClusterResourceSchedulePolicyLabelName = "multicluster.harmonycloud.cn.schedulePolicy"
)

// TODO
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
