package monitor

import (
	"context"
	"sync"
	"time"

	clusterController "harmonycloud.cn/stellaris/pkg/controller/cluster"
	corecfg "harmonycloud.cn/stellaris/pkg/core/config"

	"github.com/sirupsen/logrus"
	"harmonycloud.cn/stellaris/pkg/apis/multicluster/v1alpha1"
	multclusterclient "harmonycloud.cn/stellaris/pkg/client/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var onec sync.Once
var client *multclusterclient.Clientset

func StartCheckClusterStatus(mClient *multclusterclient.Clientset) {
	onec.Do(func() {
		client = mClient
		checkClusterStatus()
	})
}

func checkClusterStatus() {
	ctx := context.Background()

	config := corecfg.DefaultConfiguration()
	for {
		time.Sleep(config.ClusterStatusCheckPeriod)

		clusterList, err := client.MulticlusterV1alpha1().Clusters().List(ctx, metav1.ListOptions{})
		if err != nil {
			logrus.Errorf("get cluster list failed, error: %s", err)
			continue
		}

		for _, cluster := range clusterList.Items {
			if metav1.Now().Sub(cluster.Status.LastReceiveHeartBeatTimestamp.Time) >= config.OnlineExpirationTime && cluster.Status.Status == v1alpha1.OnlineStatus {
				err = policyReSchedule(ctx, &cluster)
				if err != nil {
					logrus.Errorf("change policy reSchedule failed, error: %s", err)
					continue
				}

				err = clusterController.OfflineCluster(ctx, client, &cluster)
				if err != nil {
					logrus.Errorf("change cluster(%s) status to offline failed, error: %s", cluster.GetName(), err)
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
				logrus.Errorf("update policy(%s:%s) reschedule failed, error: %s", policy.GetNamespace(), policy.GetName(), err)
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
				logrus.Errorf("get clusterset(%s) failed, error: %s", item.Name, err)
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
