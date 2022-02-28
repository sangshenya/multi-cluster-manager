package resource_aggregate_rule

import (
	"context"
	"fmt"
	"reflect"

	managerCommon "harmonycloud.cn/stellaris/pkg/common"

	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"harmonycloud.cn/stellaris/pkg/util/common"

	multclusterclient "harmonycloud.cn/stellaris/pkg/client/clientset/versioned"

	"harmonycloud.cn/stellaris/pkg/apis/multicluster/v1alpha1"
	"harmonycloud.cn/stellaris/pkg/model"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var aggregateRuleCommonLog = logf.Log.WithName("aggregate_rule_common")

func SyncAggregateRuleList(ctx context.Context, clientSet *multclusterclient.Clientset, responseType model.SyncAggregateResourceType, ruleList []v1alpha1.MultiClusterResourceAggregateRule) error {
	syncType := responseType
	existRuleMap := make(map[string]*v1alpha1.MultiClusterResourceAggregateRule)
	if responseType == model.SyncResource {
		existRuleList, err := AggregateRuleList(ctx, clientSet, metav1.NamespaceAll)
		if err != nil {
			return err
		}
		for _, existRule := range existRuleList.Items {
			key := common.NewNamespacedName(existRule.GetNamespace(), existRule.GetName()).String()
			existRuleMap[key] = &existRule
		}
		syncType = model.UpdateOrCreateResource
	}
	for _, rule := range ruleList {
		key := common.NewNamespacedName(rule.GetNamespace(), rule.GetName()).String()
		_, ok := existRuleMap[key]
		if ok {
			delete(existRuleMap, key)
		}
		err := syncAggregateRule(ctx, clientSet, syncType, &rule)
		if err != nil {
			return err
		}
	}
	if responseType == model.SyncResource && len(existRuleMap) > 0 {
		for _, v := range existRuleMap {
			err := syncAggregateRule(ctx, clientSet, model.DeleteResource, v)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func syncAggregateRule(ctx context.Context, clientSet *multclusterclient.Clientset, responseType model.SyncAggregateResourceType, rule *v1alpha1.MultiClusterResourceAggregateRule) error {
	existRule, err := clientSet.MulticlusterV1alpha1().MultiClusterResourceAggregateRules(rule.GetNamespace()).Get(ctx, rule.GetName(), metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			if responseType == model.DeleteResource {
				return nil
			}
			newRule := newAggregateRule(rule)
			newRule, err = clientSet.MulticlusterV1alpha1().MultiClusterResourceAggregateRules(rule.GetNamespace()).Create(ctx, newRule, metav1.CreateOptions{})
			if err != nil {
				aggregateRuleCommonLog.Error(err, fmt.Sprintf("sync proxy aggregate rule(%s:%s) failed", rule.GetNamespace(), rule.GetName()))
			}
			return err
		}
		aggregateRuleCommonLog.Error(err, fmt.Sprintf("sync proxy aggregate rule(%s:%s) failed", rule.GetNamespace(), rule.GetName()))
		return err
	}
	if responseType == model.DeleteResource {
		err = clientSet.MulticlusterV1alpha1().MultiClusterResourceAggregateRules(rule.GetNamespace()).Delete(ctx, rule.Name, metav1.DeleteOptions{})
		if err != nil {
			aggregateRuleCommonLog.Error(err, fmt.Sprintf("delete proxy aggregate rule(%s:%s) failed", rule.GetNamespace(), rule.GetName()))
		}
		return err
	}
	// update
	if reflect.DeepEqual(existRule.Spec, rule.Spec) {
		aggregateRuleCommonLog.Info(fmt.Sprintf("sync proxy aggregate rule(%s:%s) success, spec equal", rule.GetNamespace(), rule.GetName()))
		return nil
	}
	existRule.Spec = rule.Spec
	_, err = clientSet.MulticlusterV1alpha1().MultiClusterResourceAggregateRules(rule.GetNamespace()).Update(ctx, existRule, metav1.UpdateOptions{})
	if err != nil {
		aggregateRuleCommonLog.Error(err, fmt.Sprintf("sync proxy aggregate rule(%s:%s) failed", rule.GetNamespace(), rule.GetName()))
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

func AggregateRuleList(ctx context.Context, clientSet *multclusterclient.Clientset, ns string) (*v1alpha1.MultiClusterResourceAggregateRuleList, error) {
	return clientSet.MulticlusterV1alpha1().MultiClusterResourceAggregateRules(ns).List(ctx, metav1.ListOptions{})
}

func GetAggregateRuleListWithLabelSelector(ctx context.Context, clientSet *multclusterclient.Clientset, gvk *metav1.GroupVersionKind, ns string) (*v1alpha1.MultiClusterResourceAggregateRuleList, error) {
	targetGvkString := managerCommon.GvkLabelString(gvk)
	return clientSet.MulticlusterV1alpha1().MultiClusterResourceAggregateRules(ns).List(ctx, metav1.ListOptions{
		LabelSelector: managerCommon.AggregateResourceGvkLabelName + "=" + targetGvkString,
	})
}
