package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	table "harmonycloud.cn/stellaris/pkg/core/stream"

	timeutil "harmonycloud.cn/stellaris/pkg/util/time"

	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"harmonycloud.cn/stellaris/config"
	"harmonycloud.cn/stellaris/pkg/apis/multicluster/v1alpha1"
	clusterHealth "harmonycloud.cn/stellaris/pkg/common/cluster_health"
	clusterController "harmonycloud.cn/stellaris/pkg/controller/cluster"
	"harmonycloud.cn/stellaris/pkg/model"
	"harmonycloud.cn/stellaris/pkg/util/core"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var coreHeartbeatLog = logf.Log.WithName("core_heartbeat")

func (s *CoreServer) Heartbeat(req *config.Request, stream config.Channel_EstablishServer) {
	coreHeartbeatLog.Info(fmt.Sprintf("receive grpc request for heartbeat, cluster:%s", req.ClusterName))
	// convert data to cluster cr
	data := &model.HeartbeatWithChangeRequest{}
	err := json.Unmarshal([]byte(req.Body), data)
	if err != nil {
		coreHeartbeatLog.Error(err, "unmarshal data error")
		core.SendErrResponse(req.ClusterName, model.HeartbeatFailed, err, stream)
	}

	coreHeartbeatLog.Info("update cluster with heartbeat")
	err = s.updateClusterWithHeartbeat(req.ClusterName, data)
	if err != nil {
		coreHeartbeatLog.Error(err, "update cluster failed")
		core.SendErrResponse(req.ClusterName, model.HeartbeatFailed, err, stream)
	}

	table.Insert(req.ClusterName, &table.Stream{
		ClusterName: req.ClusterName,
		Stream:      stream,
		Status:      table.OK,
		Expire:      timeutil.NowTimeWithLoc().Add(s.Config.HeartbeatExpirePeriod * time.Second),
	})

	res := &config.Response{
		Type:        model.HeartbeatSuccess.String(),
		ClusterName: req.ClusterName,
		Body:        req.Body,
	}
	core.SendResponse(res, stream)
}

func (s *CoreServer) updateClusterWithHeartbeat(clusterName string, heartbeatRequest *model.HeartbeatWithChangeRequest) error {
	ctx := context.Background()
	cluster, err := s.mClient.MulticlusterV1alpha1().Clusters().Get(ctx, clusterName, v1.GetOptions{})
	if err != nil {
		return err
	}

	cluster, err = s.updateClusterWithHeartbeatAddons(ctx, heartbeatRequest.Addons, cluster)
	if err != nil {
		return err
	}

	return s.updateClusterStatusWithHeartbeat(ctx, cluster, heartbeatRequest.Conditions, heartbeatRequest.Healthy)
}

func (s *CoreServer) updateClusterStatusWithHeartbeat(ctx context.Context, cluster *v1alpha1.Cluster, conditions []model.Condition, healthy bool) error {
	if len(conditions) > 0 {
		clusterConditions := core.ConvertCondition2KubeCondition(conditions)
		cluster.Status.Conditions = append(cluster.Status.Conditions, clusterConditions...)
	}
	if cluster.Status.Status == v1alpha1.OfflineStatus {
		clusterConditions := clusterHealth.GenerateReadyCondition(true, healthy)
		cluster.Status.Conditions = append(cluster.Status.Conditions, clusterConditions...)
	}
	nowTime := v1.Time{Time: timeutil.NowTimeWithLoc()}
	cluster.Status.Status = v1alpha1.OnlineStatus
	cluster.Status.Healthy = healthy
	cluster.Status.LastReceiveHeartBeatTimestamp = nowTime
	cluster.Status.LastUpdateTimestamp = nowTime

	_, err := clusterController.UpdateClusterStatus(ctx, s.mClient, cluster)
	return err
}

func (s *CoreServer) updateClusterWithHeartbeatAddons(ctx context.Context, addons []model.Addon, cluster *v1alpha1.Cluster) (*v1alpha1.Cluster, error) {
	if len(addons) <= 0 {
		return cluster, nil
	}
	clusterAddons, err := core.ConvertRegisterAddons2KubeAddons(addons)
	if err != nil {
		return cluster, err
	}
	if !reflect.DeepEqual(cluster.Spec.Addons, clusterAddons) {
		cluster.Spec.Addons = clusterAddons
		cluster, err = clusterController.UpdateCluster(ctx, s.mClient, cluster)
		if err != nil {
			return cluster, err
		}
	}
	return cluster, nil
}
