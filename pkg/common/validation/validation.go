package validation

import (
	"fmt"

	kubevalidationpath "k8s.io/apimachinery/pkg/api/validation/path"
)

const clusterResourceNameMaxLength int = 48

func ValidateClusterResourceName(name string) []string {
	if len(name) > clusterResourceNameMaxLength {
		return []string{fmt.Sprintf("must be no more than %d characters", clusterResourceNameMaxLength)}
	}
	return kubevalidationpath.IsValidPathSegmentName(name)
}
