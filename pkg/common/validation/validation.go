package validation

import (
	"fmt"
	"net/url"
	"strconv"

	"harmonycloud.cn/stellaris/pkg/apis/multicluster/v1alpha1"

	kubevalidationpath "k8s.io/apimachinery/pkg/api/validation/path"
)

const clusterResourceNameMaxLength int = 100

const (
	CueIsEmpty            = "cue is empty"
	ResourceIsNil         = "binding`s resource field should not be nil"
	ResourceRefIsNil      = "resourceRef field should not be nil"
	NamePrefixedGVK       = "name must be prefixed with resourceGVK"
	CanNotChangedGVK      = "can not changed resourceGVK"
	ResourceMarshalFail   = "marshal resource failed"
	CanNotChangedIdentity = "can not changed resource identity"
)

func ValidateClusterResourceName(name string) []string {
	if len(name) > clusterResourceNameMaxLength {
		return []string{fmt.Sprintf("must be no more than %d characters", clusterResourceNameMaxLength)}
	}
	return kubevalidationpath.IsValidPathSegmentName(name)
}

func ValidateMultiSchedulePolicy(multiSchedulePolicy *v1alpha1.MultiClusterResourceSchedulePolicy) []string {
	if len(multiSchedulePolicy.Spec.Policy) == 0 && multiSchedulePolicy.Spec.OutTreePolicy == nil {
		return []string{
			"policy is empty",
		}
	}
	var errorMsgList []string
	// validate policy
	errors := validatePolicy(multiSchedulePolicy.Spec.Replicas, multiSchedulePolicy.Spec.Policy)
	if len(errors) > 0 {
		errorMsgList = append(errorMsgList, errors...)
	}
	// validate outTree policy
	errors = validateOutTreePolicy(multiSchedulePolicy.Spec.OutTreePolicy)
	if len(errors) > 0 {
		errorMsgList = append(errorMsgList, errors...)
	}
	return errorMsgList
}

func validatePolicy(totalReplicas int, schedulePolicy []v1alpha1.SchedulePolicy) []string {
	totalWeight := totalWeight(schedulePolicy)
	var errorMsgList []string
	for _, policy := range schedulePolicy {
		if policy.Min == 0 && policy.Max == 0 {
			continue
		}
		if policy.Min > policy.Max {
			errorMsgList = append(errorMsgList, "the minimum is greater than the maximum")
			continue
		}
		currentReplicas := float64(totalReplicas) * float64(policy.Weight) / float64(totalWeight)
		i, err := strconv.Atoi(fmt.Sprintf("%1.0f", currentReplicas))
		if err != nil {
			errorMsgList = append(errorMsgList, err.Error())
			continue
		}

		if !(policy.Min <= i && i <= policy.Max) {
			errorMsgList = append(errorMsgList, "currentReplicas is greater than the maximum or less then the minimum")
			continue
		}
	}
	return errorMsgList
}

func validateOutTreePolicy(outTreePolicy *v1alpha1.ScheduleOutTreePolicy) []string {
	if len(outTreePolicy.Url) == 0 {
		return []string{
			"outTreePolicy URL is empty",
		}
	}
	return ValidateClusterProxyURL(outTreePolicy.Url)
}

func totalWeight(schedulePolicy []v1alpha1.SchedulePolicy) int {
	var totalWeight int
	for _, policy := range schedulePolicy {
		totalWeight += policy.Weight
	}
	return totalWeight
}

// ValidateClusterProxyURL tests whether the proxyURL is valid.
// If not valid, a list of error string is returned. Otherwise an empty list (or nil) is returned.
func ValidateClusterProxyURL(proxyURL string) []string {
	u, err := url.Parse(proxyURL)
	if err != nil {
		return []string{fmt.Sprintf("cloud not parse: %s, %v", proxyURL, err)}
	}

	switch u.Scheme {
	case "http", "https", "socks5":
	default:
		return []string{fmt.Sprintf("unsupported scheme %q, must be http, https, or socks5", u.Scheme)}
	}

	return nil
}
