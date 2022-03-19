package resource_aggregate_policy

import (
	"context"
	"fmt"
	"reflect"

	logf "sigs.k8s.io/controller-runtime/pkg/log"

	multclusterclient "harmonycloud.cn/stellaris/pkg/client/clientset/versioned"
	"harmonycloud.cn/stellaris/pkg/utils/common"

	"harmonycloud.cn/stellaris/pkg/apis/multicluster/v1alpha1"
	"harmonycloud.cn/stellaris/pkg/model"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var aggregatePolicyCommonLog = logf.Log.WithName("aggregate_policy_common")

func SyncAggregatePolicyList(
	ctx context.Context,
	clientSet *multclusterclient.Clientset,
	responseType model.SyncAggregateResourceType,
	policyList []v1alpha1.ResourceAggregatePolicy) error {
	syncType := responseType
	existPolicyMap := make(map[string]*v1alpha1.ResourceAggregatePolicy)
	if responseType == model.SyncResource {
		existPolicyList, err := AggregatePolicyList(ctx, clientSet, metav1.NamespaceAll)
		if err != nil {
			return err
		}
		for _, existPolicy := range existPolicyList.Items {
			key := common.NewNamespacedName(existPolicy.GetNamespace(), existPolicy.GetName()).String()
			existPolicyMap[key] = &existPolicy
		}
		syncType = model.UpdateOrCreateResource
	}
	for _, policy := range policyList {
		key := common.NewNamespacedName(policy.GetNamespace(), policy.GetName()).String()
		_, ok := existPolicyMap[key]
		if ok {
			delete(existPolicyMap, key)
		}
		err := syncAggregatePolicy(ctx, clientSet, syncType, &policy)
		if err != nil {
			return err
		}
	}
	if responseType == model.SyncResource && len(existPolicyMap) > 0 {
		for _, v := range existPolicyMap {
			err := syncAggregatePolicy(ctx, clientSet, model.DeleteResource, v)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func syncAggregatePolicy(
	ctx context.Context,
	clientSet *multclusterclient.Clientset,
	responseType model.SyncAggregateResourceType,
	policy *v1alpha1.ResourceAggregatePolicy) error {
	existPolicy, err := clientSet.MulticlusterV1alpha1().ResourceAggregatePolicies(policy.GetNamespace()).Get(ctx, policy.GetName(), metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			if responseType == model.DeleteResource {
				return nil
			}
			newPolicy := newAggregatePolicy(policy)
			_, err = clientSet.MulticlusterV1alpha1().ResourceAggregatePolicies(policy.GetNamespace()).Create(ctx, newPolicy, metav1.CreateOptions{})
			if err != nil {
				aggregatePolicyCommonLog.Error(err, fmt.Sprintf("create proxy aggregate policy(%s:%s) failed", policy.GetNamespace(), policy.GetName()))
			}
			return err
		}
		aggregatePolicyCommonLog.Error(err, fmt.Sprintf("get proxy aggregate policy(%s:%s) failed", policy.GetNamespace(), policy.GetName()))
		return err
	}
	if responseType == model.DeleteResource {
		err = clientSet.MulticlusterV1alpha1().ResourceAggregatePolicies(policy.GetNamespace()).Delete(ctx, policy.Name, metav1.DeleteOptions{})
		if err != nil {
			aggregatePolicyCommonLog.Error(err, fmt.Sprintf("delete proxy aggregate policy(%s:%s) failed", policy.GetNamespace(), policy.GetName()))
		}
		return err
	}
	// update
	if reflect.DeepEqual(existPolicy.Spec, policy.Spec) {
		aggregatePolicyCommonLog.Info(fmt.Sprintf("update proxy aggregate policy(%s:%s) success, spec equal", policy.GetNamespace(), policy.GetName()))
		return nil
	}
	existPolicy.Spec = policy.Spec
	_, err = clientSet.MulticlusterV1alpha1().ResourceAggregatePolicies(policy.GetNamespace()).Update(ctx, existPolicy, metav1.UpdateOptions{})
	if err != nil {
		aggregatePolicyCommonLog.Error(err, fmt.Sprintf("sync proxy aggregate policy(%s:%s) failed", existPolicy.GetNamespace(), existPolicy.GetName()))
	}
	return err
}

func newAggregatePolicy(aggregatePolicy *v1alpha1.ResourceAggregatePolicy) *v1alpha1.ResourceAggregatePolicy {
	newPolicy := &v1alpha1.ResourceAggregatePolicy{}
	newPolicy.SetName(aggregatePolicy.GetName())
	newPolicy.SetNamespace(aggregatePolicy.GetNamespace())
	newPolicy.SetLabels(aggregatePolicy.GetLabels())
	newPolicy.Spec = aggregatePolicy.Spec
	return newPolicy
}

func AggregatePolicyList(ctx context.Context, clientSet *multclusterclient.Clientset, ns string) (*v1alpha1.ResourceAggregatePolicyList, error) {
	return clientSet.MulticlusterV1alpha1().ResourceAggregatePolicies(ns).List(ctx, metav1.ListOptions{})
}
