package utils_test

import (
	"encoding/json"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"harmonycloud.cn/stellaris/pkg/model"
	"harmonycloud.cn/stellaris/pkg/utils/core"
)

var _ = Describe("Utils", func() {
	//It("Test get gvr from unstructured object", func() {
	//	v := &unstructured.Unstructured{}
	//	err := yaml.Unmarshal([]byte(deploymentYaml), v)
	//	Expect(err).Should(BeNil())
	//	gvr := GroupVersionResourceFromUnstructured(v)
	//	expectGVR := schema.GroupVersionResource{
	//		Group:    "apps",
	//		Version:  "v1",
	//		Resource: "deployments",
	//	}
	//	Expect(gvr).Should(Equal(expectGVR))
	//})
	It("", func() {
		m := &model.HeartbeatWithChangeRequest{}
		err := json.Unmarshal([]byte(jsonString), m)
		Expect(err).Should(BeNil())
		statList, err := core.ConvertRegisterAddons2KubeAddons(m.Addons)
		fmt.Sprintln(statList, err)
	})

})

var jsonString = `{"healthy":true,"addons":[{"name":"kube-controller-manager-healthy","info":[{"type":"Pod","address":"10.10.101.205","targetRef":{"namespace":"kube-system","name":"kube-controller-manager-host-205"},"status":"Ready"}]},{"name":"kube-apiserver-healthy","info":[{"type":"Pod","address":"10.10.101.205","targetRef":{"namespace":"kube-system","name":"kube-apiserver-host-205"},"status":"Ready"}]}],"conditions":null}`

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
