package multi_cluster_resource

import (
	"context"
	"fmt"
	"reflect"

	"sigs.k8s.io/yaml"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"harmonycloud.cn/stellaris/pkg/apis/multicluster/common"
	controllerCommon "harmonycloud.cn/stellaris/pkg/controller/common"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/runtime"

	sliceutil "harmonycloud.cn/stellaris/pkg/util/slice"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"k8s.io/apimachinery/pkg/types"

	managerCommon "harmonycloud.cn/stellaris/pkg/common"

	"harmonycloud.cn/stellaris/pkg/apis/multicluster/v1alpha1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
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
        image: nginx
        ports:
        - containerPort: 80
`

var resourceYamlRef = &metav1.GroupVersionKind{
	Group:   "apps",
	Version: "v1",
	Kind:    "Deployment",
}

var (
	multiClusterResource     *v1alpha1.MultiClusterResource
	binding                  *v1alpha1.MultiClusterResourceBinding
	multiClusterResourceName string
	clusterName1             string
	clusterName2             string
	bindingName              string
	bindingNamespacedName    types.NamespacedName
	resourceNamespacedName   types.NamespacedName
	targetResource           *runtime.RawExtension
	targetResourceRef        *metav1.GroupVersionKind
)

var _ = Describe("Test multiClusterResourceController", func() {
	ctx := context.TODO()

	// 1、create multiClusterResource,then check Finalizers
	// 2、create binding and edit multiClusterResource, then check binding and clusterResource
	// 3、delete multiClusterResource,then check clusterResource alive or not

	targetResource = getResourceForYaml()
	targetResourceRef = resourceYamlRef

	multiClusterResourceName = "testresource"
	clusterName1 = "cluster1"
	clusterName2 = "cluster2"
	bindingName = "testbinding"

	multiClusterResource = &v1alpha1.MultiClusterResource{
		Spec: v1alpha1.MultiClusterResourceSpec{
			Resource:      targetResource,
			ResourceRef:   targetResourceRef,
			ReplicasField: "2",
		},
	}
	multiClusterResource.SetName(multiClusterResourceName)
	multiClusterResource.SetNamespace(managerCommon.ManagerNamespace)

	binding = &v1alpha1.MultiClusterResourceBinding{
		Spec: v1alpha1.MultiClusterResourceBindingSpec{
			Resources: []v1alpha1.MultiClusterResourceBindingResource{
				v1alpha1.MultiClusterResourceBindingResource{
					Name: multiClusterResourceName,
					Clusters: []v1alpha1.MultiClusterResourceBindingCluster{
						v1alpha1.MultiClusterResourceBindingCluster{
							Name: clusterName1,
						},
						v1alpha1.MultiClusterResourceBindingCluster{
							Name: clusterName2,
						},
					},
				},
			},
		},
	}
	binding.SetName(bindingName)
	binding.SetNamespace(managerCommon.ManagerNamespace)

	bindingNamespacedName = types.NamespacedName{
		Name:      bindingName,
		Namespace: managerCommon.ManagerNamespace,
	}

	resourceNamespacedName = types.NamespacedName{
		Namespace: managerCommon.ManagerNamespace,
		Name:      multiClusterResourceName,
	}

	// create
	It(fmt.Sprintf("create multiClusterResource(%s), check Finalizers", multiClusterResourceName), func() {
		Expect(k8sClient.Create(ctx, multiClusterResource)).Should(BeNil())
		// send event
		_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: resourceNamespacedName})
		Expect(err).Should(BeNil())
		// check Finalizers
		Expect(k8sClient.Get(ctx, resourceNamespacedName, multiClusterResource)).Should(BeNil())
		Expect(multiClusterResource.GetFinalizers()).ShouldNot(Equal(0))
		Expect(sliceutil.ContainsString(multiClusterResource.GetFinalizers(), managerCommon.FinalizerName)).Should(BeTrue())
	})

	// create binding and edit multiClusterResource
	It(fmt.Sprintf("create binding(%s) and edit multiClusterResource(%s)", bindingName, multiClusterResourceName), func() {
		createBindingAndSyncClusterResource(ctx)
		// edit multiClusterResource
		multiClusterResource.Spec.ReplicasField = "4"
		Expect(k8sClient.Update(ctx, multiClusterResource)).Should(BeNil())
		// send event
		_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: resourceNamespacedName})
		Expect(err).Should(BeNil())
		// check clusterResource
		resource, err := getResource(multiClusterResource)
		Expect(err).Should(BeNil())
		// get clusterResource
		clusterResourceList := getClusterResourceList(ctx, multiClusterResource)
		Expect(len(clusterResourceList.Items)).ShouldNot(Equal(0))

		for _, item := range clusterResourceList.Items {
			Expect(reflect.DeepEqual(item.Spec.Resource, resource)).Should(BeTrue())
		}
	})

	// delete multiClusterResource,then check clusterResource alive or not
	It(fmt.Sprintf("delete multiClusterResource(%s), then check binding(%s) and clusterResource alive or not", multiClusterResourceName, bindingName), func() {
		Expect(k8sClient.Delete(ctx, multiClusterResource)).Should(BeNil())

		// send event
		_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: resourceNamespacedName})
		Expect(err).Should(BeNil())

		// check multiClusterResource
		Expect(apierrors.IsNotFound(k8sClient.Get(ctx, resourceNamespacedName, multiClusterResource))).Should(BeTrue())

		// check binding
		Expect(apierrors.IsNotFound(k8sClient.Get(ctx, bindingNamespacedName, binding))).Should(BeTrue())

		// check clusterResource
		clusterResourceList := getClusterResourceList(ctx, multiClusterResource)
		Expect(len(clusterResourceList.Items)).Should(Equal(0))
	})

})

func getClusterResourceList(ctx context.Context, multiClusterResource *v1alpha1.MultiClusterResource) *v1alpha1.ClusterResourceList {
	clusterResourceList := &v1alpha1.ClusterResourceList{}
	selector, err := labels.Parse(managerCommon.MultiClusterResourceLabelName + "=" + multiClusterResource.Name)
	Expect(err).Should(BeNil())
	err = k8sClient.List(ctx, clusterResourceList, &client.ListOptions{
		LabelSelector: selector,
	})
	Expect(err).Should(BeNil())
	return clusterResourceList
}

func getResource(multiClusterResource *v1alpha1.MultiClusterResource) (*runtime.RawExtension, error) {
	// set resourceInfo
	// TODO if MultiClusterResourceOverride alive
	return controllerCommon.ApplyJsonPatch(multiClusterResource.Spec.Resource, []common.JSONPatch{})
}

func getResourceForYaml() *runtime.RawExtension {
	jsonData, err := yaml.YAMLToJSON([]byte(resourceYaml))
	Expect(err).Should(BeNil())
	return &runtime.RawExtension{
		Raw: jsonData,
	}
}

func createBindingAndSyncClusterResource(ctx context.Context) {
	// create binding
	Expect(k8sClient.Create(ctx, binding)).Should(BeNil())

}
