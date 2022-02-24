package config

import (
	"context"
	"regexp"

	agentconfig "harmonycloud.cn/stellaris/pkg/agent/config"
	"harmonycloud.cn/stellaris/pkg/apis/multicluster/v1alpha1"
	sliceutil "harmonycloud.cn/stellaris/pkg/util/slice"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func IsTargetResource(ctx context.Context, resourceRef *metav1.GroupVersionKind, object client.Object) bool {
	resourceConfig := informerResourceConfig.GetConfig(resourceRef)
	if resourceConfig == nil {
		return false
	}
	return limitJudgment(ctx, resourceConfig.Limit, object)
}

func limitJudgment(ctx context.Context, limit *v1alpha1.AggregatePolicyLimit, object client.Object) bool {
	if limit == nil {
		return true
	}
	if limit.Requests != nil {
		if limit.Requests.LabelsMatch != nil {
			return labelMatchResource(ctx, limit.Requests.LabelsMatch, object)
		}

		if limit.Requests.Match != nil {
			return matchResource(limit.Requests.Match, object)
		}
	}

	if limit.Ignores != nil {
		if limit.Ignores.LabelsMatch != nil {
			return !labelMatchResource(ctx, limit.Requests.LabelsMatch, object)
		}
		if limit.Ignores.Match != nil {
			return !matchResource(limit.Requests.Match, object)
		}
	}
	return true
}

// matchResource find target resource with matchList
func matchResource(matchList []v1alpha1.Match, object client.Object) bool {
	isMatch := false
	for _, match := range matchList {
		if len(match.Namespaces) <= 0 && match.NameMatch == nil {
			continue
		}
		if match.Namespaces != object.GetNamespace() {
			continue
		}
		if match.NameMatch == nil || (len(match.NameMatch.Regexp) == 0 && len(match.NameMatch.List) <= 0) {
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
func labelMatchResource(ctx context.Context, labelsMatch *v1alpha1.LabelsMatch, object client.Object) bool {
	nsList, err := targetNamespaces(ctx, labelsMatch.NamespaceSelector)
	if err != nil {
		return false
	}
	if !containsNamespace(nsList, object.GetNamespace()) {
		return false
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

func containsNamespace(nsList *v1.NamespaceList, ns string) bool {
	for _, item := range nsList.Items {
		if item.Name == ns {
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
	err = agentconfig.AgentConfig.ControllerClient.List(ctx, nsList, &client.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, err
	}
	return nsList, nil
}
