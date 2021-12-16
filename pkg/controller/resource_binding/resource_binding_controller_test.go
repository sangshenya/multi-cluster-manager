package resource_binding

import (
	"context"
	"fmt"

	"harmonycloud.cn/stellaris/pkg/apis/multicluster/common"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	sliceutil "harmonycloud.cn/stellaris/pkg/util/slice"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"harmonycloud.cn/stellaris/pkg/apis/multicluster/v1alpha1"
	managerCommon "harmonycloud.cn/stellaris/pkg/common"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Test ResourceBinding Controller", func() {

	var (
		// TODO set resource json string
		resourceJsonString string
		// TODO set active cluster`s name
		clusterName string
		resourceGvk *metav1.GroupVersionKind
	)

	ctx := context.TODO()

	resourceBinding := &v1alpha1.MultiClusterResourceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "resourceBinding",
			Namespace: managerCommon.ManagerNamespace,
		},
		Spec: v1alpha1.MultiClusterResourceBindingSpec{
			Resources: []v1alpha1.MultiClusterResourceBindingResource{},
		},
	}

	multiClusterResource := &v1alpha1.MultiClusterResource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "multiClusterResource",
			Namespace: managerCommon.ManagerNamespace,
		},
		Spec: v1alpha1.MultiClusterResourceSpec{
			Resource: &runtime.RawExtension{
				Raw: []byte(resourceJsonString),
			},
			ResourceRef: resourceGvk,
		},
	}

	BeforeEach(func() {
		Expect(k8sClient.Create(ctx, multiClusterResource)).Should(BeNil())
	})
	// create
	It(fmt.Sprintf("create binding(%s), check binding finalizers", resourceBinding.Name), func() {
		Expect(k8sClient.Create(ctx, resourceBinding)).Should(BeNil())
		bindingNamespacedName := types.NamespacedName{
			Name:      resourceBinding.GetName(),
			Namespace: resourceBinding.GetNamespace(),
		}

		_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: bindingNamespacedName})
		Expect(err).Should(BeNil())

		Expect(k8sClient.Get(ctx, bindingNamespacedName, resourceBinding)).Should(BeNil())

		Expect(resourceBinding.GetFinalizers()).ShouldNot(Equal(0))
		Expect(sliceutil.ContainsString(resourceBinding.GetFinalizers(), managerCommon.FinalizerName)).Should(BeTrue())
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
			Clusters: []v1alpha1.MultiClusterResourceBindingCluster{
				v1alpha1.MultiClusterResourceBindingCluster{
					Name: clusterName,
				},
			},
		})

		Expect(k8sClient.Update(ctx, resourceBinding)).Should(BeNil())

		_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: bindingNamespacedName})
		Expect(err).Should(BeNil())

		// check
		clusterNamespace := managerCommon.ClusterNamespace(clusterName)
		clusterResourceNamespacedName := types.NamespacedName{
			Name:      getClusterResourceName(resourceBinding.Name, multiClusterResource.Spec.ResourceRef),
			Namespace: clusterNamespace,
		}
		clusterResource := &v1alpha1.ClusterResource{}
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
		Expect(string(clusterResource.Spec.Resource.Raw)).Should(Equal(string(multiClusterResource.Spec.Resource.Raw)))

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
	// delete
	It(fmt.Sprintf("delete binding(%s), controller will delete finalizer, and delete the ClusterResource associated with binding", multiClusterResource.Name), func() {
		bindingNamespacedName := types.NamespacedName{
			Name:      resourceBinding.GetName(),
			Namespace: resourceBinding.GetNamespace(),
		}
		Expect(k8sClient.Get(ctx, bindingNamespacedName, resourceBinding)).Should(BeNil())

		Expect(k8sClient.Delete(ctx, resourceBinding)).Should(BeNil())

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
		Expect(apierrors.IsNotFound(err)).Should(BeTrue())

	})

})
