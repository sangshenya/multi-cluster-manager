package aggregate

import (
	"context"

	"harmonycloud.cn/stellaris/pkg/model"

	managerCommon "harmonycloud.cn/stellaris/pkg/common"
	"harmonycloud.cn/stellaris/pkg/utils/common"
	cueRender "harmonycloud.cn/stellaris/pkg/utils/cue-render"

	aggregateController "harmonycloud.cn/stellaris/pkg/proxy/aggregate/controller"
	"harmonycloud.cn/stellaris/pkg/proxy/aggregate/match"

	"harmonycloud.cn/stellaris/pkg/apis/multicluster/v1alpha1"
	proxy_cfg "harmonycloud.cn/stellaris/pkg/proxy/config"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// AggregateResourceWithResourceList aggregate resource when proxy register success and ruleList/policyList is not empty
func AggregateResourceWithResourceList(
	ctx context.Context,
	ruleList []v1alpha1.MultiClusterResourceAggregateRule,
	policyList []v1alpha1.ResourceAggregatePolicy) (*model.AggregateResourceDataModelList, error) {
	modelList := &model.AggregateResourceDataModelList{}
	for _, policy := range policyList {
		rList := getMatchRule(ruleList, policy)
		for _, rule := range rList {
			mList, err := AggregateResource(ctx, &rule, &policy)
			if err != nil {
				return nil, err
			}
			if len(mList.List) == 0 {
				continue
			}
			modelList.List = append(modelList.List, mList.List...)
		}
	}
	return modelList, nil
}

func AggregateResource(
	ctx context.Context,
	rule *v1alpha1.MultiClusterResourceAggregateRule,
	policy *v1alpha1.ResourceAggregatePolicy) (*model.AggregateResourceDataModelList, error) {
	modelList := &model.AggregateResourceDataModelList{}
	resourceObjectList := newResourceObject(policy.Spec.ResourceRef)
	err := proxy_cfg.ProxyConfig.ControllerClient.List(ctx, resourceObjectList)
	if err != nil {
		return nil, err
	}
	for _, resourceObject := range resourceObjectList.Items {
		if !match.IsTargetResourceWithConfig(ctx, &resourceObject, &policy.Spec) {
			continue
		}
		resourceData, err := cueRender.RenderCue(&resourceObject, rule.Spec.Rule.Cue, "")
		if err != nil {
			return nil, err
		}
		data := aggregateController.NewAggregateResourceDataRequest(rule, policy, &resourceObject, resourceData)
		if data == nil {
			continue
		}
		modelList.List = append(modelList.List, *data)
	}
	return modelList, nil
}

func getMatchRule(
	ruleList []v1alpha1.MultiClusterResourceAggregateRule,
	policy v1alpha1.ResourceAggregatePolicy) []v1alpha1.MultiClusterResourceAggregateRule {
	rList := []v1alpha1.MultiClusterResourceAggregateRule{}
	for _, rule := range ruleList {
		policyLabels := policy.GetLabels()
		ruleNamespaced, ok := policyLabels[managerCommon.AggregateRuleLabelName]
		if ok && ruleNamespaced == common.NewNamespacedName(policy.Namespace, policy.Name).String() {
			rList = append(rList, rule)
		}
	}
	return rList
}

func newResourceObject(gvk *metav1.GroupVersionKind) *unstructured.UnstructuredList {
	resourceObject := &unstructured.UnstructuredList{}
	resourceObject.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   gvk.Group,
		Version: gvk.Version,
		Kind:    gvk.Kind + "List",
	})
	return resourceObject
}
