package controller

import (
	"context"
	"fmt"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"harmonycloud.cn/stellaris/pkg/apis/multicluster/v1alpha1"
	managerCommon "harmonycloud.cn/stellaris/pkg/common"
	"harmonycloud.cn/stellaris/pkg/util/common"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("ClusterController", func() {
	var (
		clusterName = "cluster1"
	)
	ctx := context.TODO()
	cluster := &v1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterName,
		},
		Spec: v1alpha1.ClusterSpec{
			Addons: []v1alpha1.ClusterAddons{},
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
		clusterWorkspaceName, _ := common.GenerateName(cluster.Name, managerCommon.ClusterWorkspacePrefix)
		clusterWorkspaceExist := &corev1.Namespace{}
		err = k8sClient.Get(context.TODO(), types.NamespacedName{Name: clusterWorkspaceName}, clusterWorkspaceExist)
		Expect(err).ShouldNot(BeNil())
		// add finalizer
		Expect(cluster.GetFinalizers()).ShouldNot(Equal(0))
		Expect(controllerutil.ContainsFinalizer(cluster, managerCommon.ClusterControllerFinalizer)).Should(BeTrue())
	})
	It(fmt.Sprintf("update cluster(%s), check cluster finalizers", cluster.Name), func() {
		Expect(k8sClient.Create(ctx, cluster)).Should(BeNil())
		clusterNamespacedName := types.NamespacedName{
			Name: cluster.Name,
		}
		_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: clusterNamespacedName})
		Expect(err).Should(BeNil())
		// update
		Expect(k8sClient.Update(ctx, cluster)).Should(BeNil())
		_, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: clusterNamespacedName})
		Expect(err).Should(BeNil())
		// workspace should exist
		clusterWorkspaceName, _ := common.GenerateName(cluster.Name, managerCommon.ClusterWorkspacePrefix)
		clusterWorkspaceExist := &corev1.Namespace{}
		err = k8sClient.Get(context.TODO(), types.NamespacedName{Name: clusterWorkspaceName}, clusterWorkspaceExist)
		Expect(err).Should(BeNil())
		// check finalizer
		Expect(cluster.GetFinalizers()).ShouldNot(Equal(0))
		Expect(controllerutil.ContainsFinalizer(cluster, managerCommon.ClusterControllerFinalizer)).Should(BeTrue())
	})
	It(fmt.Sprintf("delete cluster(%s), check cluster finalizers", cluster.Name), func() {
		Expect(k8sClient.Create(ctx, cluster)).Should(BeNil())
		clusterNamespacedName := types.NamespacedName{
			Name: cluster.Name,
		}
		_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: clusterNamespacedName})
		Expect(err).Should(BeNil())
		// delete
		Expect(k8sClient.Delete(ctx, cluster)).Should(BeNil())
		_, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: clusterNamespacedName})
		Expect(err).Should(BeNil())
		clusterWorkspaceName, _ := common.GenerateName(cluster.Name, managerCommon.ClusterWorkspacePrefix)
		clusterWorkspaceExist := &corev1.Namespace{}
		err = k8sClient.Get(context.TODO(), types.NamespacedName{Name: clusterWorkspaceName}, clusterWorkspaceExist)
		Expect(errors.IsNotFound(err)).Should(BeTrue())

		// check finalizer
		Expect(cluster.GetFinalizers()).Should(Equal(0))
	})
})
