package controller

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"harmonycloud.cn/stellaris/pkg/apis/multicluster/v1alpha1"
	managerCommon "harmonycloud.cn/stellaris/pkg/common"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("ClusterController", func() {

	var (
		clusterName = "test1"
	)
	ctx := context.TODO()
	cluster := &v1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterName,
		},
		Spec: v1alpha1.ClusterSpec{
			Addons: []v1alpha1.ClusterAddon{},
		},
	}

	It(fmt.Sprintf("create cluster(%s), check cluster finalizers", cluster.Name), func() {
		Expect(k8sClient.Create(ctx, cluster)).Should(BeNil())
		clusterNamespacedName := types.NamespacedName{
			Name: cluster.Name,
		}
		_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: clusterNamespacedName})
		Expect(err).Should(BeNil())
		// create workspace
		clusterWorkspaceName := managerCommon.ClusterNamespace(cluster.Name)
		clusterWorkspaceExist := &corev1.Namespace{}
		err = k8sClient.Get(context.TODO(), types.NamespacedName{Name: clusterWorkspaceName}, clusterWorkspaceExist)
		Expect(err).Should(BeNil())
		// add finalizer
		createdCluster := &v1alpha1.Cluster{}
		_ = k8sClient.Get(context.TODO(), clusterNamespacedName, createdCluster)
		Expect(controllerutil.ContainsFinalizer(createdCluster, managerCommon.ClusterNamespaceInControlPlanePrefix)).Should(BeTrue())

	})
	It(fmt.Sprintf("update cluster(%s), check cluster finalizers", cluster.Name), func() {
		clusterNamespacedName := types.NamespacedName{
			Name: cluster.Name,
		}
		createdCluster := &v1alpha1.Cluster{}
		_ = k8sClient.Get(context.TODO(), clusterNamespacedName, createdCluster)

		// update
		Expect(k8sClient.Update(ctx, createdCluster)).Should(BeNil())
		_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: clusterNamespacedName})
		Expect(err).Should(BeNil())
		// workspace should exist
		clusterWorkspaceName := managerCommon.ClusterNamespace(cluster.Name)
		clusterWorkspaceExist := &corev1.Namespace{}
		err = k8sClient.Get(context.TODO(), types.NamespacedName{Name: clusterWorkspaceName}, clusterWorkspaceExist)
		Expect(err).Should(BeNil())
		// check finalizer
		createdCluster = &v1alpha1.Cluster{}
		_ = k8sClient.Get(context.TODO(), clusterNamespacedName, createdCluster)
		Expect(controllerutil.ContainsFinalizer(createdCluster, managerCommon.FinalizerName)).Should(BeTrue())
	})
	It(fmt.Sprintf("delete cluster(%s), check cluster finalizers", cluster.Name), func() {
		// Expect(k8sClient.Create(ctx, cluster)).Should(BeNil())
		clusterNamespacedName := types.NamespacedName{
			Name: cluster.Name,
		}
		// delete
		Expect(k8sClient.Delete(ctx, cluster)).Should(BeNil())
		_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: clusterNamespacedName})
		Expect(err).Should(BeNil())

		clusterWorkspaceName := managerCommon.ClusterNamespace(cluster.Name)
		clusterWorkspaceExist := &corev1.Namespace{}
		err = k8sClient.Get(context.TODO(), types.NamespacedName{Name: clusterWorkspaceName}, clusterWorkspaceExist)
		Expect(clusterWorkspaceExist.Status.Phase).Should(Equal(corev1.NamespaceTerminating))

		// check finalizer
		Expect(controllerutil.ContainsFinalizer(cluster, managerCommon.FinalizerName)).Should(BeFalse())
	})
})
