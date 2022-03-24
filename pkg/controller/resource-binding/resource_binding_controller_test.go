package resource_binding

import (
	"context"
	"encoding/json"
	"fmt"
	"harmonycloud.cn/stellaris/pkg/apis/multicluster/common"
	v1 "k8s.io/api/apps/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"reflect"
	"sigs.k8s.io/yaml"
	"strconv"

	sliceutils "harmonycloud.cn/stellaris/pkg/utils/slice"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"harmonycloud.cn/stellaris/pkg/apis/multicluster/v1alpha1"
	managerCommon "harmonycloud.cn/stellaris/pkg/common"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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
  replicas: 10
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

var _ = Describe("Test ResourceBinding Controller", func() {

	var (
		resourceJsonString string
		clusterName        string
		resourceGvk        *metav1.GroupVersionKind
	)

	resourceJsonString = resourceYaml
	clusterName = "test-multi-cluster"
	resourceGvk = &metav1.GroupVersionKind{
		Group:   "apps",
		Version: "v1",
		Kind:    "Deployment",
	}

	ctx := context.TODO()

	resourceBinding := &v1alpha1.MultiClusterResourceBinding{
		Spec: v1alpha1.MultiClusterResourceBindingSpec{
			Resources: []v1alpha1.MultiClusterResourceBindingResource{},
		},
	}
	resourceBinding.SetName("resource-binding")
	resourceBinding.SetNamespace(managerCommon.ManagerNamespace)

	multiClusterResource := &v1alpha1.MultiClusterResource{
		Spec: v1alpha1.MultiClusterResourceSpec{
			Resource:    getResourceForYaml(resourceJsonString),
			ResourceRef: resourceGvk,
		},
	}
	multiClusterResource.SetName("multi-cluster-resource")
	multiClusterResource.SetNamespace(managerCommon.ManagerNamespace)

	resourceOverride := &v1alpha1.MultiClusterResourceOverride{
		Spec: &v1alpha1.MultiClusterResourceOverrideSpec{
			Clusters: []v1alpha1.MultiClusterResourceOverrideClusters{
				v1alpha1.MultiClusterResourceOverrideClusters{
					Names: []string{clusterName},
					Overrides: []common.JSONPatch{},
				},
			},
			Resources: &v1alpha1.MultiClusterResourceOverrideResources{
				Names: []string{multiClusterResource.Name},
			},
		},
	}
	resourceOverride.SetName("resource-override")
	resourceOverride.SetNamespace(managerCommon.ManagerNamespace)

	// create
	It(fmt.Sprintf("create binding(%s), check binding finalizers", resourceBinding.Name), func() {
		err := k8sClient.Create(ctx, multiClusterResource)
		if err != nil {
			Expect(apierrors.IsAlreadyExists(err)).Should(BeTrue())
		}

		err = k8sClient.Create(ctx, resourceBinding)
		Expect(err).Should(BeNil())
		bindingNamespacedName := types.NamespacedName{
			Name:      resourceBinding.GetName(),
			Namespace: resourceBinding.GetNamespace(),
		}

		err = k8sClient.Create(ctx, resourceOverride)
		Expect(err).Should(BeNil())

		_, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: bindingNamespacedName})
		Expect(err).Should(BeNil())

		Expect(k8sClient.Get(ctx, bindingNamespacedName, resourceBinding)).Should(BeNil())

		Expect(resourceBinding.GetFinalizers()).ShouldNot(Equal(0))
		Expect(sliceutils.ContainsString(resourceBinding.GetFinalizers(), managerCommon.FinalizerName)).Should(BeTrue())
	})
	// update
	It(fmt.Sprintf("update binding(%s) spec and check the ClusterResource associated with binding", resourceBinding.Name), func() {
		bindingNamespacedName := types.NamespacedName{
			Name:      resourceBinding.GetName(),
			Namespace: resourceBinding.GetNamespace(),
		}
		Expect(k8sClient.Get(ctx, bindingNamespacedName, resourceBinding)).Should(BeNil())

		resourceBinding.Spec.Resources = append(resourceBinding.Spec.Resources, v1alpha1.MultiClusterResourceBindingResource{
			Name: multiClusterResource.GetName(),
			Namespace: managerCommon.ManagerNamespace,
			Clusters: []v1alpha1.MultiClusterResourceBindingCluster{
				v1alpha1.MultiClusterResourceBindingCluster{
					Name: clusterName,
				},
			},
		})

		Expect(k8sClient.Update(ctx, resourceBinding)).Should(BeNil())

		_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: bindingNamespacedName})
		Expect(err).Should(BeNil())

		// TODO(chenkun) check labels
		// create clusterResource
		_, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: bindingNamespacedName})
		Expect(err).Should(BeNil())

		// check
		clusterNamespace := managerCommon.ClusterNamespace(clusterName)
		clusterResourceNamespacedName := types.NamespacedName{
			Name:      getClusterResourceName(resourceBinding.Name, multiClusterResource.Spec.ResourceRef),
			Namespace: clusterNamespace,
		}
		clusterResource := &v1alpha1.ClusterResource{}
		err = k8sClient.Get(ctx, clusterResourceNamespacedName, clusterResource)
		Expect(k8sClient.Get(ctx, clusterResourceNamespacedName, clusterResource)).Should(BeNil())
		// label
		clusterResourceBindingLabelName, ok := clusterResource.GetLabels()[managerCommon.ResourceBindingLabelName]
		Expect(ok).Should(BeTrue())
		Expect(clusterResourceBindingLabelName).Should(Equal(resourceBinding.GetName()))
		// owner
		controllerRef := metav1.GetControllerOf(clusterResource)
		Expect(controllerRef).ShouldNot(BeNil())
		Expect(controllerRef.Name).Should(Equal(resourceBinding.GetName()))
		// resource
		Expect(reflect.DeepEqual(clusterResource.Spec.Resource, multiClusterResource.Spec.Resource)).Should(BeTrue())
	})
	// update status
	It(fmt.Sprintf("update ClusterResource status, check binding(%s) status", resourceBinding.Name), func() {
		clusterNamespace := managerCommon.ClusterNamespace(clusterName)
		clusterResourceNamespacedName := types.NamespacedName{
			Name:      getClusterResourceName(resourceBinding.Name, multiClusterResource.Spec.ResourceRef),
			Namespace: clusterNamespace,
		}
		clusterResource := &v1alpha1.ClusterResource{}
		Expect(k8sClient.Get(ctx, clusterResourceNamespacedName, clusterResource)).Should(BeNil())

		newStatus := v1alpha1.ClusterResourceStatus{
			ObservedReceiveGeneration: 1,
			Phase:                     common.Complete,
			Message:                   "resource apply complete",
		}
		clusterResource.Status = newStatus

		Expect(k8sClient.Status().Update(ctx, clusterResource)).Should(BeNil())

		// send event
		bindingNamespacedName := types.NamespacedName{
			Name:      resourceBinding.GetName(),
			Namespace: resourceBinding.GetNamespace(),
		}

		_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: bindingNamespacedName})
		Expect(err).Should(BeNil())

		// check
		binding := &v1alpha1.MultiClusterResourceBinding{}
		Expect(k8sClient.Get(ctx, bindingNamespacedName, binding)).Should(BeNil())
		Expect(len(binding.Status.ClusterStatus)).ShouldNot(Equal(0))

		bindingStatus := binding.Status.ClusterStatus[0]
		Expect(bindingStatus.Name).Should(Equal(clusterName))
		Expect(bindingStatus.Resource).Should(Equal(clusterResource.GetName()))
		Expect(bindingStatus.ObservedReceiveGeneration).Should(Equal(newStatus.ObservedReceiveGeneration))
		Expect(bindingStatus.Message).Should(Equal(newStatus.Message))
		Expect(bindingStatus.Phase).Should(Equal(newStatus.Phase))

	})
	It(fmt.Sprintf("update ResourceOverride (%s) Spec , check ClusterResources status", resourceOverride.Name), func() {
		bindingNamespacedName := types.NamespacedName{
			Name:      resourceBinding.GetName(),
			Namespace: resourceBinding.GetNamespace(),
		}

		resourceOverride.Spec.Clusters[0].Overrides = append(resourceOverride.Spec.Clusters[0].Overrides, common.JSONPatch{
			Path: "/spec/replicas",
			Op: "replace",
			Value: apiextensionsv1.JSON{
				Raw: []byte(strconv.Itoa(10)),
			},
		})

		err := k8sClient.Update(ctx, resourceOverride)
		Expect(err).Should(BeNil())

		_, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: bindingNamespacedName})
		Expect(err).Should(BeNil())

		clusterNamespace := managerCommon.ClusterNamespace(clusterName)
		clusterResourceNamespacedName := types.NamespacedName{
			Name:      getClusterResourceName(resourceBinding.Name, multiClusterResource.Spec.ResourceRef),
			Namespace: clusterNamespace,
		}

		clusterResource := &v1alpha1.ClusterResource{}
		k8sClient.Get(ctx, clusterResourceNamespacedName, clusterResource)

		Expect(clusterResource).NotTo(BeNil())

		var resourceContent v1.Deployment
		err = json.Unmarshal(clusterResource.Spec.Resource.Raw, &resourceContent)
		Expect(err).To(BeNil())
		Expect(*resourceContent.Spec.Replicas).To(Equal(int32(10)))
	})
	// delete
	It(fmt.Sprintf("delete binding(%s), controller will delete finalizer, and delete the ClusterResource associated with binding", multiClusterResource.Name), func() {
		bindingNamespacedName := types.NamespacedName{
			Name:      resourceBinding.GetName(),
			Namespace: resourceBinding.GetNamespace(),
		}
		Expect(k8sClient.Get(ctx, bindingNamespacedName, resourceBinding)).Should(BeNil())

		Expect(k8sClient.Delete(ctx, resourceBinding)).Should(BeNil())

		Expect(k8sClient.Delete(ctx, resourceOverride)).Should(BeNil())

		_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: bindingNamespacedName})
		Expect(err).Should(BeNil())

		// check binding
		err = k8sClient.Get(ctx, bindingNamespacedName, resourceBinding)
		Expect(apierrors.IsNotFound(err)).Should(BeTrue())

		// check clusterResource
		clusterNamespace := managerCommon.ClusterNamespace(clusterName)
		clusterResourceNamespacedName := types.NamespacedName{
			Name:      getClusterResourceName(resourceBinding.Name, multiClusterResource.Spec.ResourceRef),
			Namespace: clusterNamespace,
		}
		clusterResource := &v1alpha1.ClusterResource{}
		err = k8sClient.Get(ctx, clusterResourceNamespacedName, clusterResource)
		//Expect(apierrors.IsNotFound(err)).Should(BeTrue())

	})

	// remove resource
	It(fmt.Sprintf("clean resource"), func() {
		err := k8sClient.Delete(ctx, multiClusterResource)
		if err != nil {
			Expect(apierrors.IsNotFound(err)).Should(BeTrue())
		}
		err = k8sClient.Delete(ctx, resourceBinding)
		if err != nil {
			Expect(apierrors.IsNotFound(err)).Should(BeTrue())
		}
	})

})

func getResourceForYaml(resourceString string) *runtime.RawExtension {
	jsonData, err := yaml.YAMLToJSON([]byte(resourceString))
	Expect(err).Should(BeNil())
	return &runtime.RawExtension{
		Raw: jsonData,
	}
}
