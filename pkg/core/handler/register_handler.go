package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"harmonycloud.cn/stellaris/pkg/core/token"

	"harmonycloud.cn/stellaris/pkg/common/helper"

	clusterHealth "harmonycloud.cn/stellaris/pkg/common/cluster-health"

	"harmonycloud.cn/stellaris/pkg/controller/resource_aggregate_rule"

	resource_aggregate_policy "harmonycloud.cn/stellaris/pkg/controller/resource-aggregate-policy"

	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"harmonycloud.cn/stellaris/config"
	"harmonycloud.cn/stellaris/pkg/apis/multicluster/v1alpha1"
	managerCommon "harmonycloud.cn/stellaris/pkg/common"
	clusterController "harmonycloud.cn/stellaris/pkg/controller/cluster"
	table "harmonycloud.cn/stellaris/pkg/core/stream"
	"harmonycloud.cn/stellaris/pkg/model"
	"harmonycloud.cn/stellaris/pkg/utils/core"
	timeutils "harmonycloud.cn/stellaris/pkg/utils/time"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	ctx := context.Background()
	// validate token
	if err := token.ValidateToken(ctx, s.clientSet, data.Token); err != nil {
		coreRegisterLog.Error(err, "token validate failed")
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

	existCluster, err := s.mClient.MulticlusterV1alpha1().Clusters().Get(ctx, cluster.Name, metav1.GetOptions{})
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
		nowTime := metav1.Time{Time: timeutils.NowTimeWithLoc()}
		existCluster.Status.LastUpdateTimestamp = nowTime
		existCluster.Status.LastReceiveHeartBeatTimestamp = nowTime
		existCluster.Status.Status = v1alpha1.OnlineStatus
		existCluster.Status.Healthy = true
		conditions := clusterHealth.GenerateReadyCondition(true, true)
		if len(conditions) > 0 {
			existCluster.Status.Conditions = append(existCluster.Status.Conditions, conditions...)
		}
		_, err = clusterController.UpdateClusterStatus(ctx, s.mClient, existCluster)
		return err
	}

	_, err = s.mClient.MulticlusterV1alpha1().Clusters().Create(ctx, cluster, metav1.CreateOptions{})
	return err
}

func (s *CoreServer) getRegisterResources(clusterName string) (*model.RegisterResponse, error) {
	ctx := context.Background()
	body := &model.RegisterResponse{
		ClusterResources: make([]v1alpha1.ClusterResource, 0),
		Policies:         make([]v1alpha1.ResourceAggregatePolicy, 0),
		Rules:            make([]v1alpha1.MultiClusterResourceAggregateRule, 0),
	}
	clusterNamespace := managerCommon.ClusterNamespace(clusterName)

	var err error
	clusterResourceList, err := s.getClusterResourceList(ctx, clusterNamespace)
	if err != nil {
		coreRegisterLog.Error(err, "register response, get cluster resource list failed")
		return body, err
	}
	body.ClusterResources = clusterResourceList

	policyList, err := resource_aggregate_policy.AggregatePolicyList(ctx, s.mClient, clusterNamespace)
	if err != nil {
		coreRegisterLog.Error(err, "register response, get policy list failed")
		return body, err
	}
	for _, policy := range policyList.Items {
		mPolicyNamespace, ok := policy.GetLabels()[managerCommon.ParentResourceNamespaceLabelName]
		if !ok {
			continue
		}

		ns, err := helper.GetMappingNamespace(ctx,
			s.clientSet,
			managerCommon.ClusterName(policy.GetNamespace()),
			mPolicyNamespace)
		if err != nil {
			continue
		}
		if mPolicyNamespace != ns {
			policy.SetNamespace(ns)
		}
		body.Policies = append(body.Policies, policy)
	}

	ruleList, err := resource_aggregate_rule.AggregateRuleList(ctx, s.mClient, metav1.NamespaceAll)
	if err != nil {
		coreRegisterLog.Error(err, "register response, get rule list failed")
		return body, err
	}
	for _, rule := range ruleList.Items {
		mappingNs, err := helper.GetMappingNamespace(ctx, s.clientSet, clusterName, rule.Namespace)
		if err != nil {
			continue
		}
		if mappingNs != rule.Namespace {
			rule.Namespace = mappingNs
		}
		body.Rules = append(body.Rules, rule)
	}

	return body, nil
}

func (s *CoreServer) getClusterResourceList(ctx context.Context, clusterNamespace string) ([]v1alpha1.ClusterResource, error) {
	clusterResourceList, err := s.mClient.MulticlusterV1alpha1().ClusterResources(clusterNamespace).List(ctx, metav1.ListOptions{})
	if clusterResourceList == nil {
		return nil, err
	}
	return clusterResourceList.Items, err
}
