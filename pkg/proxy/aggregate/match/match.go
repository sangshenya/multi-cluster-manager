package match

import (
	"context"
	"regexp"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"harmonycloud.cn/stellaris/pkg/apis/multicluster/v1alpha1"
	resourceConfig "harmonycloud.cn/stellaris/pkg/proxy/aggregate/config"
	proxyconfig "harmonycloud.cn/stellaris/pkg/proxy/config"
	sliceutil "harmonycloud.cn/stellaris/pkg/utils/slice"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type PolicyType string

const (
	Requests PolicyType = "Requests"
	Ignores  PolicyType = "Ignores"
)

func IsTargetResource(ctx context.Context, resourceRef *metav1.GroupVersionKind, object *unstructured.Unstructured) bool {
	config := resourceConfig.ResourceConfig.GetConfig(resourceRef)
	if config == nil {
		return false
	}
	return limitJudgment(ctx, config.Limit, object)
}

func IsTargetResourceWithConfig(ctx context.Context, object *unstructured.Unstructured, config *v1alpha1.ResourceAggregatePolicySpec) bool {
	if config == nil {
		return false
	}
	return limitJudgment(ctx, config.Limit, object)
}

func limitJudgment(ctx context.Context, limit *v1alpha1.AggregatePolicyLimit, object *unstructured.Unstructured) bool {
	if limitIsEmpty(limit) {
		return true
	}
	if !limitRuleIsEmpty(limit.Requests) {
		return limitRuleJudgment(ctx, limit.Requests, object)
	}

	return !limitRuleJudgment(ctx, limit.Ignores, object)
}

// limitRuleJudgment Satisfy either LabelsMatch or Match
func limitRuleJudgment(ctx context.Context, limitRule *v1alpha1.AggregatePolicyLimitRule, object *unstructured.Unstructured) bool {
	isMatch := true
	if !labelsMatchIsEmpty(limitRule.LabelsMatch) {
		isMatch = labelMatchResource(ctx, limitRule.LabelsMatch, object)
	}
	return isMatch || matchResource(limitRule.Match, object)
}

// matchResource find target resource with matchList
func matchResource(matchList []v1alpha1.Match, object *unstructured.Unstructured) bool {
	isMatch := false
	for _, match := range matchList {
		if matchIsEmpty(match) {
			continue
		}
		if match.Namespaces != object.GetNamespace() {
			continue
		}
		if matchScopeIsEmpty(match.NameMatch) {
			isMatch = true
			break
		}
		if nameMatch(match.NameMatch, object.GetName()) {
			isMatch = true
			break
		}
	}
	return isMatch
}

func nameMatch(scope *v1alpha1.MatchScope, name string) bool {
	if len(scope.List) > 0 {
		return sliceutil.ContainsString(scope.List, name)
	}
	nameRe := regexp.MustCompile(scope.Regexp)
	return nameRe.MatchString(name)
}

// labelMatchResource find target resource with LabelsMatch
func labelMatchResource(ctx context.Context, labelsMatch *v1alpha1.LabelsMatch, object *unstructured.Unstructured) bool {
	if labelsMatch.NamespaceSelector != nil {
		if !containsNamespace(ctx, labelsMatch.NamespaceSelector, object) {
			return false
		}
	}
	return containsLabels(object.GetLabels(), labelsMatch.LabelSelector)
}

func containsLabels(objectLabels map[string]string, labelSelector *metav1.LabelSelector) bool {
	selector, err := metav1.LabelSelectorAsSelector(labelSelector)
	if err != nil {
		return false
	}
	return selector.Matches(labels.Set(objectLabels))
}

func containsNamespace(ctx context.Context, namespaceSelector *metav1.LabelSelector, object *unstructured.Unstructured) bool {
	nsList, err := targetNamespaces(ctx, namespaceSelector)
	if err != nil {
		return false
	}
	for _, ns := range nsList.Items {
		if ns.GetName() == object.GetNamespace() {
			return true
		}
	}
	return false
}

func targetNamespaces(ctx context.Context, namespaceSelector *metav1.LabelSelector) (*v1.NamespaceList, error) {
	labelSelector, err := metav1.LabelSelectorAsSelector(namespaceSelector)
	if err != nil {
		return nil, err
	}
	nsList := &v1.NamespaceList{}
	err = proxyconfig.ProxyConfig.ControllerClient.List(ctx, nsList, &client.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, err
	}
	return nsList, nil
}

func limitIsEmpty(limit *v1alpha1.AggregatePolicyLimit) bool {
	if limit == nil || (limitRuleIsEmpty(limit.Requests) && limitRuleIsEmpty(limit.Ignores)) {
		return true
	}
	return false
}

func limitRuleIsEmpty(limitRule *v1alpha1.AggregatePolicyLimitRule) bool {
	if limitRule == nil || (labelsMatchIsEmpty(limitRule.LabelsMatch) && matchListIsEmpty(limitRule.Match)) {
		return true
	}
	return false
}

func labelsMatchIsEmpty(labelsMatch *v1alpha1.LabelsMatch) bool {
	if labelsMatch == nil || (labelsMatch.NamespaceSelector == nil && labelsMatch.LabelSelector == nil) {
		return true
	}
	return false
}

func matchListIsEmpty(matchList []v1alpha1.Match) bool {
	if matchList == nil || len(matchList) == 0 {
		return true
	}
	isEmpty := true
	for _, match := range matchList {
		if !matchIsEmpty(match) {
			isEmpty = false
			break
		}
	}
	return isEmpty
}

func matchIsEmpty(match v1alpha1.Match) bool {
	if len(match.Namespaces) == 0 && matchScopeIsEmpty(match.NameMatch) {
		return true
	}
	return false
}

func matchScopeIsEmpty(matchScope *v1alpha1.MatchScope) bool {
	if matchScope == nil || (len(matchScope.List) == 0 && len(matchScope.Regexp) == 0) {
		return true
	}
	return false
}
