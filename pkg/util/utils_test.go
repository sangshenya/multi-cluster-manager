package utils_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "harmonycloud.cn/stellaris/pkg/util"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/yaml"
)

var _ = Describe("Utils", func() {
	It("Test get gvr from unstructured object", func() {
		v := &unstructured.Unstructured{}
		err := yaml.Unmarshal([]byte(deploymentYaml), v)
		Expect(err).Should(BeNil())
		gvr := GroupVersionResourceFromUnstructured(v)
		expectGVR := schema.GroupVersionResource{
			Group:    "apps",
			Version:  "v1",
			Resource: "deployments",
		}
		Expect(gvr).Should(Equal(expectGVR))
	})
})

var deploymentYaml = `apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    app: nginx
  name: nginx
spec:
  replicas: 2
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
      - image: nginx:latest
        name: nginx
        ports:
        - containerPort: 80
          name: nginx
          protocol: TCP
`
