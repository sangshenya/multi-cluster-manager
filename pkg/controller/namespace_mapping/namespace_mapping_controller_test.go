package namespace_mapping

import (
	"context"
	"fmt"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"harmonycloud.cn/stellaris/pkg/apis/multicluster/v1alpha1"
	managerCommon "harmonycloud.cn/stellaris/pkg/common"
	controllerCommon "harmonycloud.cn/stellaris/pkg/controller/common"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("NamespaceMappingController", func() {

	var (
		cluster = "cluster1"
		ns      = "ns1"
	)
	rule := make(map[string]string, 1)
	rule[cluster] = ns
	expectKey, _ := controllerCommon.GenerateLabelKey(cluster, ns)
	ctx := context.TODO()
	namespaceMapping := &v1alpha1.NamespaceMapping{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "namespace-mapping",
			Namespace: "mapping",
		},
		Spec: v1alpha1.NamespaceMappingSpec{
			Mapping: rule,
		},
	}

	It(fmt.Sprintf("create namespaceMapping(%s), check finalizers", namespaceMapping.Name), func() {
		Expect(k8sClient.Create(ctx, namespaceMapping)).Should(BeNil())
		mappingNamespacedName := types.NamespacedName{
			Name:      namespaceMapping.Name,
			Namespace: namespaceMapping.Namespace,
		}
		_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: mappingNamespacedName})
		Expect(err).Should(BeNil())
		workspace := &corev1.Namespace{}
		k8sClient.Get(context.TODO(), types.NamespacedName{Name: namespaceMapping.Namespace}, workspace)

		labels := workspace.GetLabels()
		Expect(labels[expectKey]).To(Equal("mapping"))

		// add finalizer
		createdNamespaceMapping := &v1alpha1.NamespaceMapping{}
		_ = k8sClient.Get(context.TODO(), mappingNamespacedName, createdNamespaceMapping)

		Expect(controllerutil.ContainsFinalizer(createdNamespaceMapping, managerCommon.NamespaceMappingControllerFinalizer)).Should(BeTrue())

	})
	It(fmt.Sprintf("update namespaceMapping(%s), check finalizers", namespaceMapping.Name), func() {
		mappingNamespacedName := types.NamespacedName{
			Name:      namespaceMapping.Name,
			Namespace: namespaceMapping.Namespace,
		}

		createdNamespaceMapping := &v1alpha1.NamespaceMapping{}
		_ = k8sClient.Get(context.TODO(), mappingNamespacedName, createdNamespaceMapping)

		// update
		ns = "ns2"
		rule[cluster] = ns
		expectKey, _ := controllerCommon.GenerateLabelKey(cluster, ns)
		ctx := context.TODO()
		createdNamespaceMapping.Spec.Mapping[cluster]=ns

		Expect(k8sClient.Update(ctx, createdNamespaceMapping)).Should(BeNil())
		_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: mappingNamespacedName})
		Expect(err).Should(BeNil())
		workspace := &corev1.Namespace{}
		k8sClient.Get(context.TODO(), types.NamespacedName{Name: namespaceMapping.Namespace}, workspace)
		labels := workspace.GetLabels()
		Expect(labels[expectKey]).To(Equal("mapping"))

		createdNamespaceMapping = &v1alpha1.NamespaceMapping{}
		_ = k8sClient.Get(context.TODO(), mappingNamespacedName, createdNamespaceMapping)
		Expect(controllerutil.ContainsFinalizer(createdNamespaceMapping, managerCommon.NamespaceMappingControllerFinalizer)).Should(BeTrue())

	})
	It(fmt.Sprintf("delete namespaceMapping(%s), check finalizers", namespaceMapping.Name), func() {
		mappingNamespacedName := types.NamespacedName{
			Name:      namespaceMapping.Name,
			Namespace: namespaceMapping.Namespace,
		}
		Expect(k8sClient.Delete(ctx, namespaceMapping)).Should(BeNil())
		_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: mappingNamespacedName})
		Expect(err).Should(BeNil())
		// check
		workspace := &corev1.Namespace{}
		k8sClient.Get(context.TODO(), types.NamespacedName{Name: namespaceMapping.Namespace}, workspace)
		labels := workspace.GetLabels()
		Expect(labels[expectKey]).Should(Equal(""))
		// check finalizer
		createdNamespaceMapping := &v1alpha1.NamespaceMapping{}
		_ = k8sClient.Get(context.TODO(), mappingNamespacedName, createdNamespaceMapping)
		Expect(controllerutil.ContainsFinalizer(createdNamespaceMapping, managerCommon.NamespaceMappingControllerFinalizer)).Should(BeFalse())


	})
})
