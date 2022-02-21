package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	agentconfig "harmonycloud.cn/stellaris/pkg/agent/config"
	"harmonycloud.cn/stellaris/pkg/apis/multicluster/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"harmonycloud.cn/stellaris/config"
	"harmonycloud.cn/stellaris/pkg/model"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var aggregateHandlerLog = logf.Log.WithName("agent_aggregate_handler")

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
		aggregateHandlerLog.Error(err, fmt.Sprintf("sync agent(%s) aggregate resource failed", response.ClusterName))
		return
	}
	ctx := context.Background()

	err = syncAggregateRuleList(ctx, response.Type, aggregateResponse.RuleList)
	if err != nil {
		aggregateHandlerLog.Error(err, fmt.Sprintf("sync agent(%s) aggregate resource failed", response.ClusterName))
		return
	}
	err = syncAggregatePolicyList(ctx, response.Type, aggregateResponse.PolicyList)
	if err != nil {
		aggregateHandlerLog.Error(err, fmt.Sprintf("sync agent(%s) aggregate resource failed", response.ClusterName))
		return
	}
}

func syncAggregatePolicyList(ctx context.Context, responseType string, policyList []*v1alpha1.ResourceAggregatePolicy) error {
	for _, policy := range policyList {
		err := syncAggregatePolicy(ctx, responseType, policy)
		if err != nil {
			return err
		}
	}
	return nil
}

func syncAggregatePolicy(ctx context.Context, responseType string, policy *v1alpha1.ResourceAggregatePolicy) error {
	existPolicy, err := agentconfig.AgentConfig.ClientV1alpha1().ResourceAggregatePolicies(policy.GetNamespace()).Get(ctx, policy.GetName(), metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			if responseType == model.AggregateDelete.String() {
				return nil
			}
			newPolicy := newAggregatePolicy(policy)
			newPolicy, err = agentconfig.AgentConfig.ClientV1alpha1().ResourceAggregatePolicies(policy.GetNamespace()).Create(ctx, newPolicy, metav1.CreateOptions{})
			if err != nil {
				aggregateHandlerLog.Error(err, fmt.Sprintf("create agent aggregate policy(%s:%s) failed", policy.GetNamespace(), policy.GetName()))
			}
			return err
		}
		aggregateHandlerLog.Error(err, fmt.Sprintf("get agent aggregate policy(%s:%s) failed", policy.GetNamespace(), policy.GetName()))
		return err
	}
	if responseType == model.AggregateDelete.String() {
		err = agentconfig.AgentConfig.ClientV1alpha1().ResourceAggregatePolicies(policy.GetNamespace()).Delete(ctx, policy.Name, metav1.DeleteOptions{})
		if err != nil {
			aggregateHandlerLog.Error(err, fmt.Sprintf("delete agent aggregate policy(%s:%s) failed", policy.GetNamespace(), policy.GetName()))
		}
		return err
	}
	// update
	if reflect.DeepEqual(existPolicy.Spec, policy.Spec) {
		aggregateHandlerLog.Info(fmt.Sprintf("update agent aggregate policy(%s:%s) success, spec equal", policy.GetNamespace(), policy.GetName()))
		return nil
	}
	existPolicy.Spec = policy.Spec
	_, err = agentconfig.AgentConfig.ClientV1alpha1().ResourceAggregatePolicies(policy.GetNamespace()).Update(ctx, existPolicy, metav1.UpdateOptions{})
	if err != nil {
		aggregateHandlerLog.Error(err, fmt.Sprintf("sync agent aggregate policy(%s:%s) failed", existPolicy.GetNamespace(), existPolicy.GetName()))
	}
	return err
}

func syncAggregateRuleList(ctx context.Context, responseType string, ruleList []*v1alpha1.MultiClusterResourceAggregateRule) error {
	for _, rule := range ruleList {
		err := syncAggregateRule(ctx, responseType, rule)
		if err != nil {
			return err
		}
	}
	return nil
}

func syncAggregateRule(ctx context.Context, responseType string, rule *v1alpha1.MultiClusterResourceAggregateRule) error {
	existRule, err := agentconfig.AgentConfig.ClientV1alpha1().MultiClusterResourceAggregateRules(rule.GetNamespace()).Get(ctx, rule.GetName(), metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			if responseType == model.AggregateDelete.String() {
				return nil
			}
			newRule := newAggregateRule(rule)
			newRule, err = agentconfig.AgentConfig.ClientV1alpha1().MultiClusterResourceAggregateRules(rule.GetNamespace()).Create(ctx, newRule, metav1.CreateOptions{})
			if err != nil {
				aggregateHandlerLog.Error(err, fmt.Sprintf("sync agent aggregate rule(%s:%s) failed", rule.GetNamespace(), rule.GetName()))
			}
			return err
		}
		aggregateHandlerLog.Error(err, fmt.Sprintf("sync agent aggregate rule(%s:%s) failed", rule.GetNamespace(), rule.GetName()))
		return err
	}
	if responseType == model.AggregateDelete.String() {
		err = agentconfig.AgentConfig.ClientV1alpha1().MultiClusterResourceAggregateRules(rule.GetNamespace()).Delete(ctx, rule.Name, metav1.DeleteOptions{})
		if err != nil {
			aggregateHandlerLog.Error(err, fmt.Sprintf("delete agent aggregate rule(%s:%s) failed", rule.GetNamespace(), rule.GetName()))
		}
		return err
	}
	// update
	if reflect.DeepEqual(existRule.Spec, rule.Spec) {
		aggregateHandlerLog.Info(fmt.Sprintf("sync agent aggregate rule(%s:%s) success, spec equal", rule.GetNamespace(), rule.GetName()))
		return nil
	}
	existRule.Spec = rule.Spec
	_, err = agentconfig.AgentConfig.ClientV1alpha1().MultiClusterResourceAggregateRules(rule.GetNamespace()).Update(ctx, existRule, metav1.UpdateOptions{})
	if err != nil {
		aggregateHandlerLog.Error(err, fmt.Sprintf("sync agent aggregate rule(%s:%s) failed", rule.GetNamespace(), rule.GetName()))
	}
	return err
}

func newAggregateRule(aggregateRule *v1alpha1.MultiClusterResourceAggregateRule) *v1alpha1.MultiClusterResourceAggregateRule {
	newRule := &v1alpha1.MultiClusterResourceAggregateRule{}
	newRule.SetName(aggregateRule.GetName())
	newRule.SetNamespace(aggregateRule.GetNamespace())
	newRule.SetLabels(aggregateRule.GetLabels())
	newRule.Spec = aggregateRule.Spec
	return newRule
}

func newAggregatePolicy(aggregatePolicy *v1alpha1.ResourceAggregatePolicy) *v1alpha1.ResourceAggregatePolicy {
	newPolicy := &v1alpha1.ResourceAggregatePolicy{}
	newPolicy.SetName(aggregatePolicy.GetName())
	newPolicy.SetNamespace(aggregatePolicy.GetNamespace())
	newPolicy.SetLabels(aggregatePolicy.GetLabels())
	newPolicy.Spec = aggregatePolicy.Spec
	return newPolicy
}
