package monitor

import (
	"context"
	"fmt"
	"sync"
	"time"

	timeutil "harmonycloud.cn/stellaris/pkg/util/time"

	logf "sigs.k8s.io/controller-runtime/pkg/log"

	clusterController "harmonycloud.cn/stellaris/pkg/controller/cluster"
	corecfg "harmonycloud.cn/stellaris/pkg/core/config"

	"harmonycloud.cn/stellaris/pkg/apis/multicluster/v1alpha1"
	multclusterclient "harmonycloud.cn/stellaris/pkg/client/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var onec sync.Once
var client *multclusterclient.Clientset
var config *corecfg.Configuration

var clusterMonitorLog = logf.Log.WithName("cluster_monitor")

func StartCheckClusterStatus(mClient *multclusterclient.Clientset, cfg *corecfg.Configuration) {
	onec.Do(func() {
		client = mClient
		config = cfg
		checkClusterStatus()
	})
}

func checkClusterStatus() {
	ctx := context.Background()
	for {
		time.Sleep(config.ClusterStatusCheckPeriod)
		clusterMonitorLog.Info("start check cluster status")
		clusterList, err := client.MulticlusterV1alpha1().Clusters().List(ctx, metav1.ListOptions{})
		if err != nil {
			clusterMonitorLog.Error(err, "get cluster list failed")
			continue
		}

		for _, cluster := range clusterList.Items {
			if timeutil.NowTimeWithLoc().Sub(cluster.Status.LastReceiveHeartBeatTimestamp.Time) >= config.OnlineExpirationTime && cluster.Status.Status == v1alpha1.OnlineStatus {
				err = policyReSchedule(ctx, &cluster)
				if err != nil {
					clusterMonitorLog.Error(err, "change policy reSchedule failed")
					continue
				}

				clusterMonitorLog.Info(fmt.Sprintf("cluster(%s) is offline, last heartBeat time:%s, now time:%s", cluster.Name, cluster.Status.LastReceiveHeartBeatTimestamp.String(), metav1.Now().String()))
				err = clusterController.OfflineCluster(ctx, client, &cluster)
				if err != nil {
					clusterMonitorLog.Error(err, fmt.Sprintf("change cluster(%s) status to offline failed", cluster.GetName()))
					continue
				}
			}
		}
	}
}

func policyReSchedule(ctx context.Context, cluster *v1alpha1.Cluster) error {
	policyList, err := client.MulticlusterV1alpha1().MultiClusterResourceSchedulePolicies(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, policy := range policyList.Items {
		if len(policy.Spec.FailoverPolicy) == 0 {
			continue
		}
		if shouldReSchedule(ctx, &policy, cluster) {
			policy.Spec.Reschedule = true
			_, err = client.MulticlusterV1alpha1().MultiClusterResourceSchedulePolicies(policy.GetNamespace()).Update(ctx, &policy, metav1.UpdateOptions{})
			if err != nil {
				clusterMonitorLog.Error(err, fmt.Sprintf("update policy(%s:%s) reschedule failed", policy.GetNamespace(), policy.GetName()))
				continue
			}
		}
	}
	return nil
}

func shouldReSchedule(ctx context.Context, policy *v1alpha1.MultiClusterResourceSchedulePolicy, cluster *v1alpha1.Cluster) bool {
	for _, item := range policy.Spec.Policy {
		if policy.Spec.ClusterSource == v1alpha1.ClusterSourceTypeAssign {
			if item.Name == cluster.GetName() {
				return true
			}
		} else if policy.Spec.ClusterSource == v1alpha1.ClusterSourceTypeClusterset {
			clusterSet, err := client.MulticlusterV1alpha1().ClusterSets().Get(ctx, policy.Spec.Clusterset, metav1.GetOptions{})
			if err != nil {
				clusterMonitorLog.Error(err, fmt.Sprintf("get clusterset(%s) failed", item.Name))
				return false
			}
			if clusterSetContainsCluster(clusterSet, cluster) {
				return true
			}
		}
	}
	return false
}

func clusterSetContainsCluster(clusterSet *v1alpha1.ClusterSet, cluster *v1alpha1.Cluster) bool {
	if len(clusterSet.Spec.Clusters) > 0 {
		for _, c := range clusterSet.Spec.Clusters {
			if c.Name == cluster.Name {
				return true
			}
		}
	} else if len(clusterSet.Spec.Selector.Labels) > 0 {
		if containsMap(cluster.GetLabels(), clusterSet.Spec.Selector.Labels) {
			return true
		}
	}
	return false
}

func containsMap(big, sub map[string]string) bool {
	for k, v := range sub {
		value, ok := big[k]
		if !ok || value != v {
			return false
		}
	}
	return true
}
