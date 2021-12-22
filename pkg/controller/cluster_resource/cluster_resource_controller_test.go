package cluster_resource

import (
	"context"
	"fmt"
	"time"

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

var (
	clusterResource               *v1alpha1.ClusterResource
	clusterResourceNamespacedName types.NamespacedName
	// TODO set resource、clusterName、resourceName
	resource     *runtime.RawExtension
	clusterName  string
	resourceName string
)

var _ = Describe("Test ControlPlane ClusterResource Controller", func() {

	ctx := context.TODO()
	clusterResource = &v1alpha1.ClusterResource{
		Spec: v1alpha1.ClusterResourceSpec{
			Resource: resource,
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

		if managerCommon.IsControlPlane() {
			// controlPlane will send ClusterResource to agent
			// agent get clusterResource will update status and send update status request to core
			time.Sleep(3 * time.Second)
			// get clusterResource
			clusterResource = getTestClusterResource(ctx)
			Expect(len(clusterResource.Status.Phase)).ShouldNot(Equal(0))
			return
		}
		// agent will update ClusterResource status
		// get clusterResource
		clusterResource = getTestClusterResource(ctx)
		Expect(clusterResource.Status.Phase).Should(Equal(common.Creating))
		// TODO check core`s clusterResource status
		Expect(clusterResource.Status.ObservedReceiveGeneration).Should(Equal(clusterResource.Generation))

		// send event,agent will create resource in agent
		_, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: clusterResourceNamespacedName})
		Expect(err).Should(BeNil())
		// get clusterResource
		clusterResource = getTestClusterResource(ctx)
		Expect(clusterResource.Status.Phase).Should(Equal(common.Complete))
		// TODO check resource should equal to clusterResource.resource
		// TODO check core`s clusterResource status

	})

	It(fmt.Sprintf("check clusterResource(%s) when delete", clusterResource.Name), func() {
		Expect(k8sClient.Delete(ctx, clusterResource)).Should(BeNil())
		// send event
		_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: clusterResourceNamespacedName})
		Expect(err).Should(BeNil())
		// check
		err = k8sClient.Get(ctx, clusterResourceNamespacedName, clusterResource)
		Expect(apierrors.IsNotFound(err)).Should(BeTrue())
		if !managerCommon.IsControlPlane() {
			// TODO check resource should IsNotFound
			return
		}
	})
})

func getTestClusterResource(ctx context.Context) *v1alpha1.ClusterResource {
	// get clusterResource
	Expect(k8sClient.Get(ctx, clusterResourceNamespacedName, clusterResource)).Should(BeNil())
	return clusterResource
}
