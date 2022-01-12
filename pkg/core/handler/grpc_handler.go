package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	clusterController "harmonycloud.cn/stellaris/pkg/controller/cluster"

	clusterHealth "harmonycloud.cn/stellaris/pkg/common/cluster_health"

	"harmonycloud.cn/stellaris/pkg/core/monitor"

	"harmonycloud.cn/stellaris/pkg/util/core"

	"github.com/sirupsen/logrus"
	"harmonycloud.cn/stellaris/config"
	"harmonycloud.cn/stellaris/pkg/apis/multicluster/v1alpha1"
	multclusterclient "harmonycloud.cn/stellaris/pkg/client/clientset/versioned"
	corecfg "harmonycloud.cn/stellaris/pkg/core/config"
	table "harmonycloud.cn/stellaris/pkg/core/stream"
	"harmonycloud.cn/stellaris/pkg/model"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type CoreServer struct {
	Handlers map[string][]Fn
	Config   *corecfg.Configuration
	mClient  *multclusterclient.Clientset
}

func NewCoreServer(cfg *corecfg.Configuration, mClient *multclusterclient.Clientset) *CoreServer {
	s := &CoreServer{Config: cfg}
	s.mClient = mClient
	s.init()
	return s
}

func (s *CoreServer) init() {
	s.Handlers = make(map[string][]Fn)
	s.registerHandler("Register", s.Register)
	s.registerHandler("Heartbeat", s.Heartbeat)
}

func (s *CoreServer) Register(req *config.Request, stream config.Channel_EstablishServer) {
	// convert data to cluster cr
	data := &model.RegisterRequest{}
	if err := json.Unmarshal([]byte(req.Body), data); err != nil {
		logrus.Errorf("unmarshal data error: %s", err)
		core.SendErrResponse(req.ClusterName, err, stream)
	}
	clusterAddons, err := core.ConvertRegisterAddons2KubeAddons(data.Addons)
	if err != nil {
		logrus.Errorf("cannot convert request to cluster resource", err)
		core.SendErrResponse(req.ClusterName, err, stream)
	}

	cluster := &v1alpha1.Cluster{
		ObjectMeta: v1.ObjectMeta{
			Name: req.ClusterName,
		},
		Spec: v1alpha1.ClusterSpec{
			Addons: clusterAddons,
		},
	}

	// create or update cluster resource in k8s
	if err := s.registerClusterInKube(cluster); err != nil {
		logrus.Errorf("cannot register cluster %s in k8s", err)
		core.SendErrResponse(req.ClusterName, err, stream)
	}

	// start check cluster status
	monitor.StartCheckClusterStatus(s.mClient)

	// write stream into stream table
	if err := table.Insert(req.ClusterName, &table.Stream{
		ClusterName: req.ClusterName,
		Stream:      stream,
		Status:      table.OK,
		Expire:      time.Now().Add(s.Config.HeartbeatExpirePeriod * time.Second),
	}); err != nil {
		logrus.Error("insert stream table error: %s", err)
		core.SendErrResponse(req.ClusterName, err, stream)
	}

	res := &config.Response{
		Type:        "RegisterSuccess",
		ClusterName: req.ClusterName,
	}
	core.SendResponse(res, stream)
}

func (s *CoreServer) Heartbeat(req *config.Request, stream config.Channel_EstablishServer) {
	// convert data to cluster cr
	data := &model.HeartbeatWithChangeRequest{}
	err := json.Unmarshal([]byte(req.Body), data)
	if err != nil {
		logrus.Errorf("unmarshal data error: %s", err)
		core.SendErrResponse(req.ClusterName, err, stream)
	}

	err = s.updateClusterWithHeartbeat(req.ClusterName, data)
	if err != nil {
		logrus.Errorf("update cluster failed: %s", err)
		core.SendErrResponse(req.ClusterName, err, stream)
	}
	// TODO update proxy monitor data and refresh stream table status

	res := &config.Response{
		Type:        "HeartbeatSuccess",
		ClusterName: req.ClusterName,
		Body:        "",
	}
	core.SendResponse(res, stream)
}

func (s *CoreServer) registerHandler(typ string, fn Fn) {
	fns := s.Handlers[typ]
	if fns == nil {
		fns = make([]Fn, 0, 5)
	}
	fns = append(fns, fn)
	s.Handlers[typ] = fns
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
	nowTime := v1.Now()
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

func (s *CoreServer) registerClusterInKube(cluster *v1alpha1.Cluster) error {
	ctx := context.Background()
	update := true

	existCluster, err := s.mClient.MulticlusterV1alpha1().Clusters().Get(ctx, cluster.Name, v1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			update = false
		} else {
			return err
		}
	}
	nowTime := v1.Now()
	createStatus := v1alpha1.ClusterStatus{
		Conditions:                    clusterHealth.GenerateReadyCondition(true, true),
		LastReceiveHeartBeatTimestamp: nowTime,
		LastUpdateTimestamp:           nowTime,
		Healthy:                       true,
		Status:                        v1alpha1.OnlineStatus,
	}
	if update {
		if cluster.Status.Status == v1alpha1.OnlineStatus {
			return fmt.Errorf("cluster %s is online now", cluster.Name)
		}
		existCluster.Spec = cluster.Spec
		existCluster, err = clusterController.UpdateCluster(ctx, s.mClient, existCluster)
		if err != nil {
			return err
		}
		// update cluster status
		existCluster.Status = createStatus
		_, err = clusterController.UpdateClusterStatus(ctx, s.mClient, existCluster)
		return err
	}

	if _, err := s.mClient.MulticlusterV1alpha1().Clusters().Create(ctx, cluster, v1.CreateOptions{}); err != nil {
		return err
	}

	// update cluster status
	cluster.Status = createStatus
	_, err = clusterController.UpdateClusterStatus(ctx, s.mClient, cluster)
	return err
}
