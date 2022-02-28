package handler

import (
	"context"
	"encoding/json"
	"fmt"

	proxy_cfg "harmonycloud.cn/stellaris/pkg/proxy/config"

	resource_aggregate_policy "harmonycloud.cn/stellaris/pkg/controller/resource-aggregate-policy"

	"harmonycloud.cn/stellaris/pkg/controller/resource_aggregate_rule"

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
