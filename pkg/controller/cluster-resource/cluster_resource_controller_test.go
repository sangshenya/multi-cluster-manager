package cluster_resource

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"sigs.k8s.io/yaml"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"harmonycloud.cn/stellaris/pkg/apis/multicluster/common"
	sliceutil "harmonycloud.cn/stellaris/pkg/util/slice"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"harmonycloud.cn/stellaris/pkg/apis/multicluster/v1alpha1"
	managerCommon "harmonycloud.cn/stellaris/pkg/common"
	"k8s.io/apimachinery/pkg/runtime"
)

var resourceYaml = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-nginx-app
  namespace: chenkun
spec:
  selector:
    matchLabels:
      run: my-nginx-app
  replicas: 2
  template:
    metadata:
      labels:
        run: my-nginx-app
    spec:
      containers:
      - name: my-nginx-app
        image: crccheck/hello-world
        ports:
        - containerPort: 8000
`

var resourceYamlRef = metav1.GroupVersionKind{
	Group:   "apps",
	Version: "v1",
	Kind:    "Deployment",
}

var (
	clusterResource               *v1alpha1.ClusterResource
	clusterResourceNamespacedName types.NamespacedName
	clusterName                   string
	resourceName                  string
)

var _ = Describe("Test ControlPlane ClusterResource Controller", func() {

	ctx := context.TODO()

	It("clusterResource", func() {
		_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: "binding-test.apps.v1.deployment", Namespace: "stellaris-harmonycloud-cn-cluster238"}})
		Expect(err).Should(BeNil())
	})

	clusterName = "test-multi-cluster"
	resourceName = "test-resource"
	clusterResource = &v1alpha1.ClusterResource{
		Spec: v1alpha1.ClusterResourceSpec{
			Resource: getResourceForYaml(resourceYaml),
		},
	}
	clusterResource.SetName(resourceName)
	clusterResource.SetNamespace(managerCommon.ClusterNamespace(clusterName))

	clusterResourceNamespacedName.Name = resourceName
	clusterResourceNamespacedName.Namespace = managerCommon.ClusterNamespace(clusterName)

	It(fmt.Sprintf("create clusterResource(%s), check clusterResource finalizers", clusterResource.Name), func() {
		Expect(k8sClient.Create(ctx, clusterResource)).Should(BeNil())
		// send event and check error
		_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: clusterResourceNamespacedName})
		Expect(err).Should(BeNil())

		// get clusterResource
		clusterResource = getTestClusterResource(ctx)

		// check finalizers
		Expect(clusterResource.GetFinalizers()).ShouldNot(Equal(0))
		Expect(sliceutil.ContainsString(clusterResource.GetFinalizers(), managerCommon.FinalizerName)).Should(BeTrue())
	})

	It(fmt.Sprintf("check the resource associated with ClusterResource(%s) in agent, check the ClusterResource associated with ClusterResource(%s) in controlPlane", clusterResource.Name, clusterResource.Name), func() {
		// send event and check error
		_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: clusterResourceNamespacedName})
		Expect(err).Should(BeNil())

		if reconciler.isControlPlane {
			// controlPlane will send ClusterResource to agent
			// agent get clusterResource will update status and send update status request to core
			agentSendUpdateStatusRequestToCore(ctx, v1alpha1.ClusterResourceStatus{
				ObservedReceiveGeneration: 1,
				Phase:                     common.Complete,
				Message:                   "resource apply complete",
			})

			// get clusterResource
			clusterResource = getTestClusterResource(ctx)
			Expect(len(clusterResource.Status.Phase)).ShouldNot(Equal(0))
			return
		}
		// agent will update ClusterResource status
		// get clusterResource
		clusterResource = getTestClusterResource(ctx)
		Expect(clusterResource.Status.Phase).Should(Equal(common.Creating))
		Expect(clusterResource.Status.ObservedReceiveGeneration).Should(Equal(clusterResource.Generation))

		// send event,agent will create resource in agent
		_, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: clusterResourceNamespacedName})
		Expect(err).Should(BeNil())
		// get clusterResource
		clusterResource = getTestClusterResource(ctx)
		Expect(clusterResource.Status.Phase).Should(Equal(common.Complete))

		unObject := FormatDataToUnstructured(clusterResource.Spec.Resource)
		un := &unstructured.Unstructured{}
		un.SetGroupVersionKind(schema.GroupVersionKind{
			Kind:    resourceYamlRef.Kind,
			Group:   resourceYamlRef.Group,
			Version: resourceYamlRef.Version,
		})
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name:      unObject.GetName(),
			Namespace: unObject.GetNamespace(),
		}, un)).Should(BeNil())

	})

	It(fmt.Sprintf("check clusterResource(%s) when delete", clusterResource.Name), func() {
		Expect(k8sClient.Delete(ctx, clusterResource)).Should(BeNil())
		// send event
		_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: clusterResourceNamespacedName})
		Expect(err).Should(BeNil())
		// check
		err = k8sClient.Get(ctx, clusterResourceNamespacedName, clusterResource)
		Expect(apierrors.IsNotFound(err)).Should(BeTrue())
		if !reconciler.isControlPlane {
			unObject := FormatDataToUnstructured(clusterResource.Spec.Resource)

			un := &unstructured.Unstructured{}
			un.SetGroupVersionKind(schema.GroupVersionKind{
				Kind:    resourceYamlRef.Kind,
				Group:   resourceYamlRef.Group,
				Version: resourceYamlRef.Version,
			})
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      unObject.GetName(),
				Namespace: unObject.GetNamespace(),
			}, un)
			Expect(apierrors.IsNotFound(err)).Should(BeTrue())
			return
		}
	})
})

func getTestClusterResource(ctx context.Context) *v1alpha1.ClusterResource {
	// get clusterResource
	Expect(k8sClient.Get(ctx, clusterResourceNamespacedName, clusterResource)).Should(BeNil())
	return clusterResource
}

func agentSendUpdateStatusRequestToCore(ctx context.Context, status v1alpha1.ClusterResourceStatus) {
	clusterResource = getTestClusterResource(ctx)
	clusterResource.Status = status
	Expect(k8sClient.Status().Update(ctx, clusterResource)).Should(BeNil())
}

func FormatDataToUnstructured(resource *runtime.RawExtension) *unstructured.Unstructured {
	un := &unstructured.Unstructured{}
	err := un.UnmarshalJSON(resource.Raw)
	Expect(err).Should(BeNil())
	return un
}

func getResourceForYaml(resourceString string) *runtime.RawExtension {
	jsonData, err := yaml.YAMLToJSON([]byte(resourceString))
	Expect(err).Should(BeNil())
	return &runtime.RawExtension{
		Raw: jsonData,
	}
}
