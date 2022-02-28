package config

import (
	"fmt"
	"testing"

	v1 "k8s.io/api/core/v1"
)

func TestResourceList(t *testing.T) {
	podList := &v1.PodList{}
	fmt.Println(podList.Kind)
}
