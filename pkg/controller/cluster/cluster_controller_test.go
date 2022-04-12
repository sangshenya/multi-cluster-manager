package controller

import (
	"context"
	"encoding/json"

	"harmonycloud.cn/stellaris/pkg/model"
	"harmonycloud.cn/stellaris/pkg/utils/core"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"harmonycloud.cn/stellaris/pkg/apis/multicluster/v1alpha1"
	managerCommon "harmonycloud.cn/stellaris/pkg/common"
	"k8s.io/apimachinery/pkg/types"
)

var jsonString = `{"healthy":true,"addons":[{"name":"kube-controller-manager-healthy","info":[{"type":"Pod","address":"10.10.101.205","targetRef":{"namespace":"kube-system","name":"kube-controller-manager-host-205"},"status":"Ready"}]},{"name":"kube-apiserver-healthy","info":[{"type":"Pod","address":"10.10.101.205","targetRef":{"namespace":"kube-system","name":"kube-apiserver-host-205"},"status":"Ready"}]}],"conditions":null}`

var _ = Describe("ClusterController", func() {

	var (
		clusterName = "example-test-1"
	)
	ctx := context.Background()

	It("cccc", func() {
		m := &model.HeartbeatWithChangeRequest{}
		err := json.Unmarshal([]byte(jsonString), m)
		Expect(err).Should(BeNil())
		statList, err := core.ConvertRegisterAddons2KubeAddons(m.Addons)
		Expect(err).Should(BeNil())
		clusterNamespacedName := types.NamespacedName{
			Name:      clusterName,
			Namespace: managerCommon.ClusterNamespace(clusterName),
		}
		cluster := &v1alpha1.Cluster{}
		err = k8sClient.Get(ctx, clusterNamespacedName, cluster)
		Expect(err).Should(BeNil())
		cluster.Status.Addons = statList
		err = k8sClient.Status().Update(ctx, cluster)
		Expect(err).Should(BeNil())
	})
	//var (
	//	clusterName = "test1"
	//)
	//ctx := context.TODO()
	//cluster := &v1alpha1.Cluster{
	//	ObjectMeta: metav1.ObjectMeta{
	//		Name: clusterName,
	//	},
	//	Spec: v1alpha1.ClusterSpec{
	//		Addons: []v1alpha1.ClusterAddon{},
	//	},
	//}
	//
	//It(fmt.Sprintf("create cluster(%s), check cluster finalizers", cluster.Name), func() {
	//	Expect(k8sClient.Create(ctx, cluster)).Should(BeNil())
	//	clusterNamespacedName := types.NamespacedName{
	//		Name: cluster.Name,
	//	}
	//	_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: clusterNamespacedName})
	//	Expect(err).Should(BeNil())
	//	// create workspace
	//	clusterWorkspaceName := managerCommon.ClusterNamespace(cluster.Name)
	//	clusterWorkspaceExist := &corev1.Namespace{}
	//	err = k8sClient.Get(context.TODO(), types.NamespacedName{Name: clusterWorkspaceName}, clusterWorkspaceExist)
	//	Expect(err).Should(BeNil())
	//	// add finalizer
	//	createdCluster := &v1alpha1.Cluster{}
	//	_ = k8sClient.Get(context.TODO(), clusterNamespacedName, createdCluster)
	//	Expect(controllerutil.ContainsFinalizer(createdCluster, managerCommon.ClusterNamespaceInControlPlanePrefix)).Should(BeTrue())
	//
	//})
	//It(fmt.Sprintf("update cluster(%s), check cluster finalizers", cluster.Name), func() {
	//	clusterNamespacedName := types.NamespacedName{
	//		Name: cluster.Name,
	//	}
	//	createdCluster := &v1alpha1.Cluster{}
	//	_ = k8sClient.Get(context.TODO(), clusterNamespacedName, createdCluster)
	//
	//	// update
	//	Expect(k8sClient.Update(ctx, createdCluster)).Should(BeNil())
	//	_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: clusterNamespacedName})
	//	Expect(err).Should(BeNil())
	//	// workspace should exist
	//	clusterWorkspaceName := managerCommon.ClusterNamespace(cluster.Name)
	//	clusterWorkspaceExist := &corev1.Namespace{}
	//	err = k8sClient.Get(context.TODO(), types.NamespacedName{Name: clusterWorkspaceName}, clusterWorkspaceExist)
	//	Expect(err).Should(BeNil())
	//	// check finalizer
	//	createdCluster = &v1alpha1.Cluster{}
	//	_ = k8sClient.Get(context.TODO(), clusterNamespacedName, createdCluster)
	//	Expect(controllerutil.ContainsFinalizer(createdCluster, managerCommon.FinalizerName)).Should(BeTrue())
	//})
	//It(fmt.Sprintf("delete cluster(%s), check cluster finalizers", cluster.Name), func() {
	//	// Expect(k8sClient.Create(ctx, cluster)).Should(BeNil())
	//	clusterNamespacedName := types.NamespacedName{
	//		Name: cluster.Name,
	//	}
	//	// delete
	//	Expect(k8sClient.Delete(ctx, cluster)).Should(BeNil())
	//	_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: clusterNamespacedName})
	//	Expect(err).Should(BeNil())
	//
	//	clusterWorkspaceName := managerCommon.ClusterNamespace(cluster.Name)
	//	clusterWorkspaceExist := &corev1.Namespace{}
	//	err = k8sClient.Get(context.TODO(), types.NamespacedName{Name: clusterWorkspaceName}, clusterWorkspaceExist)
	//	Expect(clusterWorkspaceExist.Status.Phase).Should(Equal(corev1.NamespaceTerminating))
	//
	//	// check finalizer
	//	Expect(controllerutil.ContainsFinalizer(cluster, managerCommon.FinalizerName)).Should(BeFalse())
	//})
})
