package handler

import (
	"context"
	"encoding/json"
	"fmt"

	proxysend "harmonycloud.cn/stellaris/pkg/proxy/send"

	proxyAggregate "harmonycloud.cn/stellaris/pkg/proxy/aggregate"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"harmonycloud.cn/stellaris/pkg/apis/multicluster/v1alpha1"

	proxy_cfg "harmonycloud.cn/stellaris/pkg/proxy/config"

	resource_aggregate_policy "harmonycloud.cn/stellaris/pkg/controller/resource-aggregate-policy"
	"harmonycloud.cn/stellaris/pkg/controller/resource_aggregate_rule"
	utils "harmonycloud.cn/stellaris/pkg/utils/common"

	"harmonycloud.cn/stellaris/config"
	"harmonycloud.cn/stellaris/pkg/model"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var aggregateHandlerLog = logf.Log.WithName("proxy_aggregate_handler")

func RecvSyncAggregateResponse(response *config.Response) {
	aggregateHandlerLog.Info(fmt.Sprintf("recv aggregate response form core: %s", response.String()))
	switch response.Type {
	case model.AggregateUpdateOrCreate.String():
		syncAggregateResource(response)
	case model.AggregateDelete.String():
		syncAggregateResource(response)
	}
}

func syncAggregateResource(response *config.Response) {
	aggregateResponse := &model.SyncAggregateResourceModel{}
	err := json.Unmarshal([]byte(response.Body), aggregateResponse)
	if err != nil {
		aggregateHandlerLog.Error(err, fmt.Sprintf("sync proxy(%s) aggregate resource failed", response.ClusterName))
		return
	}
	ctx := context.Background()
	aggregateType := changeResponseTypeToAggregateType(response.Type)
	if aggregateType == model.UnknownType {
		return
	}

	if len(aggregateResponse.PolicyList) > 0 {
		err = aggregateResource(ctx, aggregateResponse.PolicyList)
		if err != nil {
			aggregateHandlerLog.Error(err, fmt.Sprintf("aggregate resource failed, cluster(%s)", response.ClusterName))
		}
	}

	err = resource_aggregate_rule.SyncAggregateRuleList(ctx, proxy_cfg.ProxyConfig.ProxyClient, aggregateType, aggregateResponse.RuleList)
	if err != nil {
		aggregateHandlerLog.Error(err, fmt.Sprintf("sync proxy(%s) aggregate resource failed", response.ClusterName))
		return
	}

	err = resource_aggregate_policy.SyncAggregatePolicyList(ctx, proxy_cfg.ProxyConfig.ProxyClient, aggregateType, aggregateResponse.PolicyList)
	if err != nil {
		aggregateHandlerLog.Error(err, fmt.Sprintf("sync proxy(%s) aggregate resource failed", response.ClusterName))
		return
	}
}

func aggregateResource(ctx context.Context, policyList []v1alpha1.ResourceAggregatePolicy) error {
	modelList := &model.AggregateResourceDataModelList{}
	for _, policy := range policyList {
		ruleList, err := utils.GetAggregateRuleListWithLabelSelector(ctx, proxy_cfg.ProxyConfig.ProxyClient, policy.Spec.ResourceRef, metav1.NamespaceAll)
		if err != nil {
			return err
		}
		for _, rule := range ruleList.Items {
			mList, err := proxyAggregate.AggregateResource(ctx, &rule, &policy)
			if err != nil {
				return err
			}
			if len(mList.List) <= 0 {
				continue
			}
			modelList.List = append(modelList.List, mList.List...)
		}
	}
	data, err := json.Marshal(modelList)
	if err != nil {
		return err
	}
	request, err := proxysend.NewAggregateRequest(proxy_cfg.ProxyConfig.Cfg.ClusterName, string(data))
	if err != nil {
		return err
	}
	err = proxysend.SendSyncAggregateRequest(request)
	if err != nil {
		return err
	}
	return nil
}
