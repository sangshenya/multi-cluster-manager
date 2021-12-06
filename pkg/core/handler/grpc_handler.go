package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"harmonycloud.cn/multi-cluster-manager/config"
	"harmonycloud.cn/multi-cluster-manager/pkg/apis/multicluster/v1alpha1"
	multclusterclient "harmonycloud.cn/multi-cluster-manager/pkg/client/clientset/versioned"
	corecfg "harmonycloud.cn/multi-cluster-manager/pkg/core/config"
	table "harmonycloud.cn/multi-cluster-manager/pkg/core/stream"
	"harmonycloud.cn/multi-cluster-manager/pkg/core/utils"
	"harmonycloud.cn/multi-cluster-manager/pkg/model"
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
		utils.SendErrResponse(req.ClusterName, err, stream)
	}
	clusterAddons, err := utils.ConvertRegisterAddons2KubeAddons(data.Addons)
	if err != nil {
		logrus.Errorf("cannot convert request to cluster resource", err)
		utils.SendErrResponse(req.ClusterName, err, stream)
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
		utils.SendErrResponse(req.ClusterName, err, stream)
	}

	// write stream into stream table
	if err := table.Insert(req.ClusterName, &table.Stream{
		ClusterName: req.ClusterName,
		Stream:      stream,
		Status:      table.OK,
		Expire:      time.Now().Add(s.Config.HeartbeatExpirePeriod * time.Second),
	}); err != nil {
		logrus.Error("insert stream table error: %s", err)
		utils.SendErrResponse(req.ClusterName, err, stream)
	}

	res := &config.Response{
		Type:        "RegisterSuccess",
		ClusterName: req.ClusterName,
	}
	utils.SendResponse(res, stream)
}

func (s *CoreServer) Heartbeat(req *config.Request, stream config.Channel_EstablishServer) {
	// TODO update proxy monitor data and refresh stream table status

	res := &config.Response{
		Type:        "HeartbeatSuccess",
		ClusterName: req.ClusterName,
		Body:        "",
	}
	utils.SendResponse(res, stream)
}

func (s *CoreServer) registerHandler(typ string, fn Fn) {
	fns := s.Handlers[typ]
	if fns == nil {
		fns = make([]Fn, 0, 5)
	}
	fns = append(fns, fn)
	s.Handlers[typ] = fns
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

	if update {
		if cluster.Status.Status == v1alpha1.OnlineStatus {
			return fmt.Errorf("cluster %s is online now", cluster.Name)
		}
		existCluster.Spec = cluster.Spec
		if _, err := s.mClient.MulticlusterV1alpha1().Clusters().Update(ctx, existCluster, v1.UpdateOptions{}); err != nil {
			return err
		}
	} else {
		if _, err := s.mClient.MulticlusterV1alpha1().Clusters().Create(ctx, cluster, v1.CreateOptions{}); err != nil {
			return err
		}
	}

	return nil
}
