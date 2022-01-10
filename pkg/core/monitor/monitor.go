package monitor

import (
	"context"
	"sync"
	"time"

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
			if metav1.Now().Sub(cluster.Status.LastReceiveHeartBeatTimestamp.Time) >= config.OnlineExpirationTime {
				err = offlineCluster(ctx, client, &cluster)
				if err != nil {
					logrus.Errorf("change cluster status to offline failed, error: %s", err)
					continue
				}
			}
		}
	}
}

func offlineCluster(ctx context.Context, client *multclusterclient.Clientset, cluster *v1alpha1.Cluster) error {
	cluster.Status.Status = v1alpha1.OfflineStatus
	cluster.Status.LastUpdateTimestamp = metav1.Now()
	_, err := client.MulticlusterV1alpha1().Clusters().UpdateStatus(ctx, cluster, metav1.UpdateOptions{})
	return err
}

func policyReSchedule(cluster *v1alpha1.Cluster) error {

	return nil
}
