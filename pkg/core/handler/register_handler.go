package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"harmonycloud.cn/stellaris/config"
	"harmonycloud.cn/stellaris/pkg/apis/multicluster/v1alpha1"
	managerCommon "harmonycloud.cn/stellaris/pkg/common"
	clusterHealth "harmonycloud.cn/stellaris/pkg/common/cluster-health"
	clusterController "harmonycloud.cn/stellaris/pkg/controller/cluster"
	table "harmonycloud.cn/stellaris/pkg/core/stream"
	"harmonycloud.cn/stellaris/pkg/model"
	"harmonycloud.cn/stellaris/pkg/utils/core"
	timeutils "harmonycloud.cn/stellaris/pkg/utils/time"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var coreRegisterLog = logf.Log.WithName("core_register")

func (s *CoreServer) Register(req *config.Request, stream config.Channel_EstablishServer) {
	coreRegisterLog.Info(fmt.Sprintf("receive grpc request for register, cluster:%s, type:%s", req.ClusterName, req.Type))
	// convert data to cluster cr
	data := &model.RegisterRequest{}
	if err := json.Unmarshal([]byte(req.Body), data); err != nil {
		coreRegisterLog.Error(err, "unmarshal data error")
		core.SendErrResponse(req.ClusterName, model.RegisterFailed, err, stream)
	}
	clusterAddons, err := core.ConvertRegisterAddons2KubeAddons(data.Addons)
	if err != nil {
		coreRegisterLog.Error(err, "cannot convert request to cluster resource")
		core.SendErrResponse(req.ClusterName, model.RegisterFailed, err, stream)
	}
	// new cluster
	cluster := core.NewCluster(req.ClusterName)
	cluster.Status.Addons = clusterAddons

	// create or update cluster resource in k8s
	if err = s.registerClusterInKube(cluster); err != nil {
		coreRegisterLog.Error(err, fmt.Sprintf("register cluster(%s) failed", cluster.Name))
		core.SendErrResponse(req.ClusterName, model.RegisterFailed, err, stream)
	}
	coreRegisterLog.Info(fmt.Sprintf("register cluster(%s) success", cluster.Name))

	// write stream into stream table
	table.Insert(req.ClusterName, &table.Stream{
		ClusterName: req.ClusterName,
		Stream:      stream,
		Status:      table.OK,
		Expire:      timeutils.NowTimeWithLoc().Add(s.Config.HeartbeatExpirePeriod * time.Second),
	})

	res := s.newResponse(req.ClusterName)
	core.SendResponse(res, stream)
}

func (s *CoreServer) newResponse(clusterName string) *config.Response {
	res := &config.Response{
		Type:        model.RegisterSuccess.String(),
		ClusterName: clusterName,
	}
	// body
	body, _ := s.getRegisterResources(clusterName)
	if !body.IsEmpty() {
		bodyData, err := json.Marshal(body)
		if err == nil {
			res.Body = string(bodyData)
		}
	}
	return res
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
			coreRegisterLog.Info(fmt.Sprintf("cluster %s is online now", cluster.Name))
			return nil
		}
		existCluster.Spec = cluster.Spec
		existCluster, err = clusterController.UpdateCluster(ctx, s.mClient, existCluster)
		if err != nil {
			return err
		}
		// update cluster status
		nowTime := v1.Time{Time: timeutils.NowTimeWithLoc()}
		existCluster.Status.LastUpdateTimestamp = nowTime
		existCluster.Status.LastReceiveHeartBeatTimestamp = nowTime
		existCluster.Status.Status = v1alpha1.OnlineStatus
		existCluster.Status.Healthy = true
		existCluster.Status.Conditions = append(existCluster.Status.Conditions, clusterHealth.GenerateReadyCondition(true, true)...)
		_, err = clusterController.UpdateClusterStatus(ctx, s.mClient, existCluster)
		return err
	}

	_, err = s.mClient.MulticlusterV1alpha1().Clusters().Create(ctx, cluster, v1.CreateOptions{})
	return err
}

func (s *CoreServer) getRegisterResources(clusterName string) (*model.RegisterResponse, error) {
	ctx := context.Background()
	body := &model.RegisterResponse{}
	clusterNamespace := managerCommon.ClusterNamespace(clusterName)

	clusterResourceList, err := s.getClusterResourceList(ctx, clusterNamespace)
	if err != nil {
		coreRegisterLog.Error(err, "register response, get cluster resource list failed")
		return body, err
	}
	body.ClusterResources = clusterResourceList

	policyList, err := s.getPolicyList(ctx, clusterNamespace)
	if err != nil {
		coreRegisterLog.Error(err, "register response, get policy list failed")
		return body, err
	}
	body.MultiClusterResourceAggregatePolicies = policyList

	ruleList, err := s.getRuleList(ctx)
	if err != nil {
		coreRegisterLog.Error(err, "register response, get rule list failed")
		return body, err
	}
	body.MultiClusterResourceAggregateRules = ruleList

	return body, nil
}

func (s *CoreServer) getClusterResourceList(ctx context.Context, clusterNamespace string) ([]string, error) {
	var itemList []string
	clusterResourceList, err := s.mClient.MulticlusterV1alpha1().ClusterResources(clusterNamespace).List(ctx, v1.ListOptions{})
	if err != nil {
		return itemList, err
	}
	for _, item := range clusterResourceList.Items {
		itemData, err := json.Marshal(item)
		if err != nil {
			break
		}
		itemList = append(itemList, string(itemData))
	}
	return itemList, nil
}

func (s *CoreServer) getPolicyList(ctx context.Context, clusterNamespace string) ([]string, error) {
	var itemList []string
	policyList, err := s.mClient.MulticlusterV1alpha1().MultiClusterResourceAggregatePolicies(clusterNamespace).List(ctx, v1.ListOptions{})
	if err != nil {
		return itemList, err
	}
	for _, item := range policyList.Items {
		itemData, err := json.Marshal(item)
		if err != nil {
			break
		}
		itemList = append(itemList, string(itemData))
	}
	return itemList, nil
}

func (s *CoreServer) getRuleList(ctx context.Context) ([]string, error) {
	var itemList []string
	ruleList, err := s.mClient.MulticlusterV1alpha1().MultiClusterResourceAggregateRules(v1.NamespaceAll).List(ctx, v1.ListOptions{})
	if err != nil {
		return itemList, err
	}
	for _, item := range ruleList.Items {
		itemData, err := json.Marshal(item)
		if err != nil {
			break
		}
		itemList = append(itemList, string(itemData))
	}
	return itemList, nil
}
