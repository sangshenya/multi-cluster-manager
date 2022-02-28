package multi_cluster_resource

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"harmonycloud.cn/stellaris/pkg/controller/resource-binding"

	"sigs.k8s.io/yaml"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"harmonycloud.cn/stellaris/pkg/apis/multicluster/common"
	controllerCommon "harmonycloud.cn/stellaris/pkg/controller/common"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/runtime"

	sliceutils "harmonycloud.cn/stellaris/pkg/utils/slice"
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
        image: crccheck/hello-world
        ports:
        - containerPort: 8000
`
var resourceYamlImageChanged = `
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
	bindingName              string
	bindingNamespacedName    types.NamespacedName
	resourceNamespacedName   types.NamespacedName
	targetResource           *runtime.RawExtension
	targetResourceRef        *metav1.GroupVersionKind
)

var _ = Describe("Test multiClusterResourceController", func() {
	ctx := context.TODO()

	It("1111", func() {
		list, err := getBindingList(ctx)
		if err != nil {
			fmt.Println(err)
		}
		fmt.Println(len(list.Items))
	})

	// 1、create multiClusterResource,then check Finalizers
	// 2、create binding and edit multiClusterResource, then check binding and clusterResource
	// 3、delete multiClusterResource,then check clusterResource alive or not

	targetResource = getResourceForYaml(resourceYaml)
	targetResourceRef = resourceYamlRef

	multiClusterResourceName = "testresource"
	clusterName1 = "test-multi-cluster"
	bindingName = "testbinding"

	multiClusterResource = &v1alpha1.MultiClusterResource{
		Spec: v1alpha1.MultiClusterResourceSpec{
			Resource:      targetResource,
			ResourceRef:   targetResourceRef,
			ReplicasField: "2",
		},
	}
	multiClusterResource.SetNamespace(managerCommon.ManagerNamespace)
	multiClusterResource.SetName(multiClusterResourceName)

	binding = &v1alpha1.MultiClusterResourceBinding{
		Spec: v1alpha1.MultiClusterResourceBindingSpec{
			Resources: []v1alpha1.MultiClusterResourceBindingResource{
				v1alpha1.MultiClusterResourceBindingResource{
					Name: multiClusterResourceName,
					Clusters: []v1alpha1.MultiClusterResourceBindingCluster{
						v1alpha1.MultiClusterResourceBindingCluster{
							Name: clusterName1,
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
		Expect(sliceutils.ContainsString(multiClusterResource.GetFinalizers(), managerCommon.FinalizerName)).Should(BeTrue())
		Expect(multiClusterResource.Spec.Resource).Should(Equal(getResourceForYaml(resourceYaml)))
	})

	// create binding and edit multiClusterResource
	It(fmt.Sprintf("create binding(%s) and edit multiClusterResource(%s)", bindingName, multiClusterResourceName), func() {
		// create binding and clusterResource
		createBindingAndSyncClusterResource(ctx)

		time.Sleep(5 * time.Second)
		Expect(k8sClient.Get(ctx, resourceNamespacedName, multiClusterResource)).Should(BeNil())
		// edit multiClusterResource，update image
		multiClusterResource.Spec.Resource = getResourceForYaml(resourceYamlImageChanged)
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

		deleteBindingFinalizers(ctx)

		time.Sleep(5 * time.Second)

		// check multiClusterResource
		Expect(apierrors.IsNotFound(k8sClient.Get(ctx, resourceNamespacedName, multiClusterResource))).Should(BeTrue())

		// check binding
		Expect(apierrors.IsNotFound(k8sClient.Get(ctx, bindingNamespacedName, binding))).Should(BeTrue())

		// check clusterResource
		clusterResourceList := getClusterResourceList(ctx, multiClusterResource)
		Expect(len(clusterResourceList.Items)).Should(Equal(0))
	})

})

func getBindingList(ctx context.Context) (*v1alpha1.MultiClusterResourceBindingList, error) {
	str := managerCommon.MultiClusterResourceLabelName + "." + "apps.v1.deployment.resource" + "=1"
	selector, err := labels.Parse(str)
	if err != nil {
		return nil, err
	}
	bindingList := &v1alpha1.MultiClusterResourceBindingList{}
	err = k8sClient.List(ctx, bindingList, &client.ListOptions{
		LabelSelector: selector,
	})
	return bindingList, err
}

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

func getResourceForYaml(resourceString string) *runtime.RawExtension {
	jsonData, err := yaml.YAMLToJSON([]byte(resourceString))
	Expect(err).Should(BeNil())
	return &runtime.RawExtension{
		Raw: jsonData,
	}
}

func createBindingAndSyncClusterResource(ctx context.Context) {
	labelKey := managerCommon.MultiClusterResourceLabelName + "." + multiClusterResourceName
	binding.SetLabels(map[string]string{
		labelKey: "1",
	})
	// create binding
	Expect(k8sClient.Create(ctx, binding)).Should(BeNil())

	binding.Finalizers = append(binding.Finalizers, managerCommon.FinalizerName)
	Expect(k8sClient.Update(ctx, binding)).Should(BeNil())

	Expect(resource_binding.SyncClusterResourceWithBinding(ctx, k8sClient, binding)).Should(BeNil())
}

func deleteBindingFinalizers(ctx context.Context) {
	Expect(k8sClient.Get(ctx, bindingNamespacedName, binding)).Should(BeNil())
	if sliceutils.ContainsString(binding.Finalizers, managerCommon.FinalizerName) && !binding.GetDeletionTimestamp().IsZero() {
		binding.Finalizers = sliceutils.RemoveString(binding.Finalizers, managerCommon.FinalizerName)
		Expect(k8sClient.Update(ctx, binding)).Should(BeNil())
	}
}
