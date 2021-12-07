package resource_binding_controller_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestResourceBindingController(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ResourceBindingController Suite")
}
