package validation

import (
	"fmt"

	kubevalidationpath "k8s.io/apimachinery/pkg/api/validation/path"
)

const clusterResourceNameMaxLength int = 100

const (
	CueIsEmpty       = "cue is empty"
	ResourceIsNil    = "resource field should not be nil"
	ResourceRefIsNil = "resourceRef field should not be nil"
	NamePrefixedGVK  = "name must be prefixed with resourceGVK"
)

func ValidateClusterResourceName(name string) []string {
	if len(name) > clusterResourceNameMaxLength {
		return []string{fmt.Sprintf("must be no more than %d characters", clusterResourceNameMaxLength)}
	}
	return kubevalidationpath.IsValidPathSegmentName(name)
}
