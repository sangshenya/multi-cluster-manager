package controller

import (
	"context"

	"harmonycloud.cn/stellaris/pkg/apis/multicluster/v1alpha1"
	multclusterclient "harmonycloud.cn/stellaris/pkg/client/clientset/versioned"
	clusterHealth "harmonycloud.cn/stellaris/pkg/common/cluster-health"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	ConditionMaximumLength = 10
)

func OfflineCluster(ctx context.Context, client *multclusterclient.Clientset, cluster *v1alpha1.Cluster) error {
	if cluster.Status.Status == v1alpha1.OfflineStatus {
		return nil
	}
	cluster.Status.Status = v1alpha1.OfflineStatus
	cluster.Status.LastUpdateTimestamp = metav1.Now()
	conditions := clusterHealth.GenerateReadyCondition(false, false)
	cluster.Status.Conditions = append(cluster.Status.Conditions, conditions...)

	_, err := UpdateClusterStatus(ctx, client, cluster)
	return err
}

func UpdateClusterStatus(ctx context.Context, client *multclusterclient.Clientset, cluster *v1alpha1.Cluster) (*v1alpha1.Cluster, error) {
	if len(cluster.Status.Conditions) > ConditionMaximumLength {
		cluster.Status.Conditions = cluster.Status.Conditions[len(cluster.Status.Conditions)-ConditionMaximumLength:]
	}
	return client.MulticlusterV1alpha1().Clusters().UpdateStatus(ctx, cluster, metav1.UpdateOptions{})
}

func UpdateCluster(ctx context.Context, client *multclusterclient.Clientset, cluster *v1alpha1.Cluster) (*v1alpha1.Cluster, error) {
	return client.MulticlusterV1alpha1().Clusters().Update(ctx, cluster, metav1.UpdateOptions{})
}
