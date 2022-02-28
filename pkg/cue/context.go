package cue

import (
	"encoding/json"
	"fmt"

	"harmonycloud.cn/stellaris/pkg/apis/multicluster/v1alpha1"
)

type Context interface {
	GetContextFile() string
}

type ClusterContext struct {
	ClusterName string `json:"clusterName"`
}

func NewClusterContext(cluster *v1alpha1.Cluster) *ClusterContext {
	return &ClusterContext{
		ClusterName: cluster.Name,
	}
}

func (c *ClusterContext) GetContextFile() string {
	result := "context: {}"
	b, err := json.Marshal(c)
	if err != nil || string(b) == "null" {
		return result
	}
	return fmt.Sprintf("%s: %s", "context", string(b))
}
