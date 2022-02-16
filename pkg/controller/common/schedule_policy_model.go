package common

import "harmonycloud.cn/stellaris/pkg/apis/multicluster/v1alpha1"

type FailoverPolicyIndex struct {
	DoneIndex                       int
	ClusterSetIndex                 int
	UnavailableFailoverClusterIndex int
	FailoverIndex                   int
}

type SortPolicy struct {
	SortPolicyList      []v1alpha1.SchedulePolicy
	SortPolicyListIndex int
}

type FirstReplaceReplicasModel struct {
	ResourceName string
	TotalWeight int
	DiffReplicas int
}