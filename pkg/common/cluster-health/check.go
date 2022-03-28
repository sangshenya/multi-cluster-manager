package cluster_health

import (
	"context"
	"net/http"

	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"harmonycloud.cn/stellaris/pkg/apis/multicluster/common"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	clientset "harmonycloud.cn/stellaris/pkg/client/clientset/versioned"
)

var clusterHealthLog = logf.Log.WithName("cluster_health_check")

const (
	clusterReady              = "ClusterReady"
	clusterHealthy            = "cluster is reachable and health endpoint responded with ok"
	clusterNotReady           = "ClusterNotReady"
	clusterUnhealthy          = "cluster is reachable but health endpoint responded without ok"
	clusterNotReachableReason = "ClusterNotReachable"
	clusterNotReachableMsg    = "cluster is not reachable"
)

func GetClusterHealthStatus(client *clientset.Clientset) (online, healthy bool) {
	healthStatus, err := healthEndpointCheck(client, "/healthz")

	if err != nil {
		clusterHealthLog.Error(err, "Failed to do cluster health check")
		return false, false
	}

	if healthStatus != http.StatusOK {
		clusterHealthLog.Info("current cluster isn't healthy")
		return true, false
	}

	return true, true
}

func healthEndpointCheck(client *clientset.Clientset, path string) (int, error) {
	var healthStatus int
	resp := client.DiscoveryClient.RESTClient().Get().AbsPath(path).Do(context.TODO()).StatusCode(&healthStatus)
	return healthStatus, resp.Error()
}

func GenerateReadyCondition(online, healthy bool) []common.Condition {
	var conditions []common.Condition
	currentTime := metav1.Now()

	newClusterOfflineCondition := common.Condition{
		Timestamp: currentTime,
		Type:      "Ready",
		Reason:    clusterNotReachableReason,
		Message:   clusterNotReachableMsg,
	}

	newClusterReadyCondition := common.Condition{
		Timestamp: currentTime,
		Type:      "Ready",
		Reason:    clusterReady,
		Message:   clusterHealthy,
	}

	newClusterNotReadyCondition := common.Condition{
		Timestamp: currentTime,
		Type:      "Ready",
		Reason:    clusterNotReady,
		Message:   clusterUnhealthy,
	}

	if !online {
		conditions = append(conditions, newClusterOfflineCondition)
	} else {
		if !healthy {
			conditions = append(conditions, newClusterNotReadyCondition)
		} else {
			conditions = append(conditions, newClusterReadyCondition)
		}
	}

	return conditions
}
