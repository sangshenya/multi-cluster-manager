package cluster_set

import (
	"context"
	"fmt"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"harmonycloud.cn/stellaris/pkg/apis/multicluster/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("ClusterSet", func() {
	ctx := context.TODO()
	clusterSet := &v1alpha1.ClusterSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cluster-set",
		},
		Spec: v1alpha1.ClusterSetSpec{
			Selector: v1alpha1.ClusterSetSelector{},
			Clusters: []v1alpha1.ClusterSetTarget{},
			Policy:   "policy",
		},
	}
	BeforeEach(func() {

	})
	It(fmt.Sprintf("create clusterSet(%s)", clusterSet.Name), func() {
		Expect(k8sClient.Create(ctx, clusterSet)).Should(BeNil())
		clusterSetNamespacedName := types.NamespacedName{
			Name: clusterSet.Name,
		}
		_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: clusterSetNamespacedName})
		Expect(err).Should(BeNil())
	})
	It(fmt.Sprintf("update clusterSet(%s)", clusterSet.Name), func() {
		clusterSetNamespacedName := types.NamespacedName{
			Name: clusterSet.Name,
		}
		createdClusterSet := &v1alpha1.ClusterSet{}
		_ = k8sClient.Get(context.TODO(), clusterSetNamespacedName, createdClusterSet)

		Expect(k8sClient.Update(ctx, createdClusterSet)).Should(BeNil())
		_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: clusterSetNamespacedName})
		Expect(err).Should(BeNil())
	})
	It(fmt.Sprintf("delete clusterSet(%s)", clusterSet.Name), func() {
		clusterSetNamespacedName := types.NamespacedName{
			Name: clusterSet.Name,
		}

		Expect(k8sClient.Delete(ctx, clusterSet)).Should(BeNil())
		_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: clusterSetNamespacedName})
		Expect(err).Should(BeNil())
	})

})
