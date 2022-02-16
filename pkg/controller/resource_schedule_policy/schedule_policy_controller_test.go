package resource_schedule_policy

import (
	"context"
	"fmt"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"harmonycloud.cn/stellaris/pkg/apis/multicluster/v1alpha1"
	pkgcommon "harmonycloud.cn/stellaris/pkg/common"
	"harmonycloud.cn/stellaris/pkg/util/common"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"time"
)

var _ = Describe("SchedulePolicyController", func() {
	var (
		policy         []v1alpha1.SchedulePolicy
		resource       []v1alpha1.SchedulePolicyResource
		failoverPolicy []v1alpha1.ScheduleFailoverPolicy
	)
	ctx := context.TODO()
	Describe(fmt.Sprintf("Duplicated"), func() {
		It(fmt.Sprintf("normal case"), func() {
			policy = []v1alpha1.SchedulePolicy{}
			resource = []v1alpha1.SchedulePolicyResource{}
			// cluster1,cluster2,cluster3
			clusterList := &v1alpha1.ClusterList{}
			k8sClient.List(ctx, clusterList)
			for _, item := range clusterList.Items {
				if item.Name == "cluster1" || item.Name == "cluster2" || item.Name == "cluster3" {
					status := v1alpha1.ClusterStatus{
						LastReceiveHeartBeatTimestamp: metav1.Time{Time: time.Now()}, LastUpdateTimestamp: metav1.Time{Time: time.Now()}, Status: v1alpha1.OnlineStatus,
					}
					item.Status = status
					Expect(k8sClient.Status().Update(ctx, &item)).Should(BeNil())
				}
			}
			policy = append(policy, v1alpha1.SchedulePolicy{Name: "cluster1"})
			policy = append(policy, v1alpha1.SchedulePolicy{Name: "cluster2"})
			policy = append(policy, v1alpha1.SchedulePolicy{Name: "cluster3"})
			resource = append(resource, v1alpha1.SchedulePolicyResource{Name: "apps.v1.deployment.nginx"})
			policyTest := &v1alpha1.MultiClusterResourceSchedulePolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "schedule-test"},
				Spec: v1alpha1.MultiClusterResourceSchedulePolicySpec{Resources: resource,
					ClusterSource: v1alpha1.ClusterSourceTypeAssign,
					Replicas:      5,
					ScheduleMode:  v1alpha1.ScheduleModeTypeDuplicated,
					Reschedule:    true,
					Policy:        policy,
				},
			}
			Expect(k8sClient.Create(ctx, policyTest)).Should(BeNil())
			policyNamespacedName := types.NamespacedName{
				Name:      policyTest.Name,
				Namespace: policyTest.Namespace,
			}
			// reconcile
			_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: policyNamespacedName})
			Expect(err).Should(BeNil())
			createdBinding := &v1alpha1.MultiClusterResourceBinding{}
			bindingName, _ := common.GenerateNameByOption(pkgcommon.Scheduler, policyTest.Name, "-")
			k8sClient.Get(context.TODO(), types.NamespacedName{Name: bindingName, Namespace: policyTest.Namespace}, createdBinding)
			Expect(createdBinding).ShouldNot(BeNil())
			k8sClient.Delete(ctx, policyTest)
			k8sClient.Delete(ctx, createdBinding)
		})
		It(fmt.Sprintf("failover"), func() {
			failoverPolicy = []v1alpha1.ScheduleFailoverPolicy{}
			policy = []v1alpha1.SchedulePolicy{}
			resource = []v1alpha1.SchedulePolicyResource{}
			clusterList := &v1alpha1.ClusterList{}
			k8sClient.List(ctx, clusterList)
			for _, item := range clusterList.Items {
				if item.Name == "cluster4" || item.Name == "cluster6" {
					status := v1alpha1.ClusterStatus{
						LastReceiveHeartBeatTimestamp: metav1.Time{Time: time.Now()}, LastUpdateTimestamp: metav1.Time{Time: time.Now()}, Status: v1alpha1.OnlineStatus,
					}
					item.Status = status
					Expect(k8sClient.Status().Update(ctx, &item)).Should(BeNil())
				}
				if item.Name == "cluster1" || item.Name == "cluster2" || item.Name == "cluster3" || item.Name == "cluster5" {
					status := v1alpha1.ClusterStatus{
						LastReceiveHeartBeatTimestamp: metav1.Time{Time: time.Now()}, LastUpdateTimestamp: metav1.Time{Time: time.Now()}, Status: v1alpha1.OfflineStatus,
					}
					item.Status = status
					Expect(k8sClient.Status().Update(ctx, &item)).Should(BeNil())
				}
			}
			policy = append(policy, v1alpha1.SchedulePolicy{Name: "cluster1"})
			policy = append(policy, v1alpha1.SchedulePolicy{Name: "cluster2"})
			failoverPolicy = append(failoverPolicy, v1alpha1.ScheduleFailoverPolicy{Name: "cluster3", Type: "clusters"})
			// cluster4,5,6
			failoverPolicy = append(failoverPolicy, v1alpha1.ScheduleFailoverPolicy{Name: "cluster-set1", Type: "clusterset"})
			resource = append(resource, v1alpha1.SchedulePolicyResource{Name: "apps.v1.deployment.nginx"})
			policyTest := &v1alpha1.MultiClusterResourceSchedulePolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "schedule-test"},
				Spec: v1alpha1.MultiClusterResourceSchedulePolicySpec{Resources: resource,
					ClusterSource:  v1alpha1.ClusterSourceTypeAssign,
					Replicas:       5,
					ScheduleMode:   v1alpha1.ScheduleModeTypeDuplicated,
					Reschedule:     true,
					Policy:         policy,
					FailoverPolicy: failoverPolicy,
				},
			}
			Expect(k8sClient.Create(ctx, policyTest)).Should(BeNil())
			policyNamespacedName := types.NamespacedName{
				Name:      policyTest.Name,
				Namespace: policyTest.Namespace,
			}
			// reconcile
			_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: policyNamespacedName})
			Expect(err).Should(BeNil())
			createdBinding := &v1alpha1.MultiClusterResourceBinding{}
			bindingName, _ := common.GenerateNameByOption(pkgcommon.Scheduler, policyTest.Name, "-")
			k8sClient.Get(context.TODO(), types.NamespacedName{Name: bindingName, Namespace: policyTest.Namespace}, createdBinding)

			Expect(createdBinding.Spec.Resources[0].Clusters[0].Name).Should(Equal("cluster4"))
			Expect(createdBinding.Spec.Resources[0].Clusters[1].Name).Should(Equal("cluster6"))

			k8sClient.Delete(ctx, policyTest)
			k8sClient.Delete(ctx, createdBinding)
		})
	})
	Describe(fmt.Sprintf("Weighted"), func() {
		It(fmt.Sprintf("sample case"), func() {
			policy = []v1alpha1.SchedulePolicy{}
			resource = []v1alpha1.SchedulePolicyResource{}
			clusterList := &v1alpha1.ClusterList{}
			k8sClient.List(ctx, clusterList)
			for _, item := range clusterList.Items {
				if item.Name == "cluster1" || item.Name == "cluster2" || item.Name == "cluster3" {
					status := v1alpha1.ClusterStatus{
						LastReceiveHeartBeatTimestamp: metav1.Time{Time: time.Now()}, LastUpdateTimestamp: metav1.Time{Time: time.Now()}, Status: v1alpha1.OnlineStatus,
					}

					item.Status = status
					Expect(k8sClient.Status().Update(ctx, &item)).Should(BeNil())
				}
			}
			// replicas should be 7,6,7
			policy = append(policy, v1alpha1.SchedulePolicy{Name: "cluster1", Weight: 5, Min: 3, Max: 15})
			policy = append(policy, v1alpha1.SchedulePolicy{Name: "cluster2", Weight: 8, Min: 4, Max: 6})
			policy = append(policy, v1alpha1.SchedulePolicy{Name: "cluster3", Weight: 5, Min: 3, Max: 15})
			resource = append(resource, v1alpha1.SchedulePolicyResource{Name: "apps.v1.deployment.nginx"})
			policyTest := &v1alpha1.MultiClusterResourceSchedulePolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "schedule-test"},
				Spec: v1alpha1.MultiClusterResourceSchedulePolicySpec{Resources: resource,
					ClusterSource: v1alpha1.ClusterSourceTypeAssign,
					Replicas:      20,
					ScheduleMode:  v1alpha1.ScheduleModeTypeWeighted,
					Reschedule:    true,
					Policy:        policy,
				},
			}

			Expect(k8sClient.Create(ctx, policyTest)).Should(BeNil())
			policyNamespacedName := types.NamespacedName{
				Name:      policyTest.Name,
				Namespace: policyTest.Namespace,
			}
			// reconcile
			_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: policyNamespacedName})
			Expect(err).Should(BeNil())
			createdBinding := &v1alpha1.MultiClusterResourceBinding{}
			bindingName, _ := common.GenerateNameByOption(pkgcommon.Scheduler, policyTest.Name, "-")
			k8sClient.Get(context.TODO(), types.NamespacedName{Name: bindingName, Namespace: policyTest.Namespace}, createdBinding)
			Expect(createdBinding.Spec.Resources[0].Clusters[0].Override[0].Value).Should(Equal("7"))
			Expect(createdBinding.Spec.Resources[0].Clusters[1].Override[0].Value).Should(Equal("6"))
			Expect(createdBinding.Spec.Resources[0].Clusters[2].Override[0].Value).Should(Equal("7"))
			k8sClient.Delete(ctx, policyTest)
			k8sClient.Delete(ctx, createdBinding)

		})
	})
	Describe(fmt.Sprintf("clusterSet"), func() {
		It(fmt.Sprintf("clusterRole"), func() {
			policy = []v1alpha1.SchedulePolicy{}
			resource = []v1alpha1.SchedulePolicyResource{}
			clusterList := &v1alpha1.ClusterList{}
			k8sClient.List(ctx, clusterList)
			for _, item := range clusterList.Items {
				if item.Name == "cluster1" || item.Name == "cluster2" || item.Name == "cluster3" {
					status := v1alpha1.ClusterStatus{
						LastReceiveHeartBeatTimestamp: metav1.Time{Time: time.Now()}, LastUpdateTimestamp: metav1.Time{Time: time.Now()}, Status: v1alpha1.OnlineStatus,
					}
					item.Status = status
					Expect(k8sClient.Status().Update(ctx, &item)).Should(BeNil())
				}
			}
			// replicas should be 5,7,8
			policy = append(policy, v1alpha1.SchedulePolicy{Role: "master", Weight: 1, Min: 5, Max: 10})
			policy = append(policy, v1alpha1.SchedulePolicy{Role: "slave", Weight: 5, Min: 5, Max: 10})
			resource = append(resource, v1alpha1.SchedulePolicyResource{Name: "apps.v1.deployment.nginx"})
			policyTest := &v1alpha1.MultiClusterResourceSchedulePolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "schedule-test"},
				Spec: v1alpha1.MultiClusterResourceSchedulePolicySpec{Resources: resource,
					ClusterSource: v1alpha1.ClusterSourceTypeClusterset,
					Replicas:      20,
					// cluster1,2,3
					Clusterset: "cluster-set",
					Reschedule: true,
					Policy:     policy,
				},
			}

			Expect(k8sClient.Create(ctx, policyTest)).Should(BeNil())
			policyNamespacedName := types.NamespacedName{
				Name:      policyTest.Name,
				Namespace: policyTest.Namespace,
			}
			// reconcile
			_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: policyNamespacedName})
			Expect(err).Should(BeNil())
			createdBinding := &v1alpha1.MultiClusterResourceBinding{}
			bindingName, _ := common.GenerateNameByOption(pkgcommon.Scheduler, policyTest.Name, "-")
			k8sClient.Get(context.TODO(), types.NamespacedName{Name: bindingName, Namespace: policyTest.Namespace}, createdBinding)
			Expect(createdBinding.Spec.Resources[0].Clusters[0].Override[0].Value).Should(Equal("5"))
			Expect(createdBinding.Spec.Resources[0].Clusters[1].Override[0].Value).Should(Equal("7"))
			Expect(createdBinding.Spec.Resources[0].Clusters[2].Override[0].Value).Should(Equal("8"))
			k8sClient.Delete(ctx, policyTest)
			k8sClient.Delete(ctx, createdBinding)
		})
	})
	It(fmt.Sprintf("cluster selector"), func() {
		policy = []v1alpha1.SchedulePolicy{}
		resource = []v1alpha1.SchedulePolicyResource{}
		clusterList := &v1alpha1.ClusterList{}
		k8sClient.List(ctx, clusterList)
		for _, item := range clusterList.Items {
			if item.Name == "cluster1" || item.Name == "cluster2" || item.Name == "cluster3" {
				status := v1alpha1.ClusterStatus{
					LastReceiveHeartBeatTimestamp: metav1.Time{Time: time.Now()}, LastUpdateTimestamp: metav1.Time{Time: time.Now()}, Status: v1alpha1.OnlineStatus,
				}
				item.Status = status
				Expect(k8sClient.Status().Update(ctx, &item)).Should(BeNil())
			}
			if item.Name == "cluster1" || item.Name == "cluster2"{
				labels := item.Labels
				if labels == nil {
					labels = make(map[string]string)
				}
				labels["test"] ="selector"
				item.SetLabels(labels)
			}
			Expect(k8sClient.Update(ctx,&item)).Should(BeNil())

		}
		resource = append(resource, v1alpha1.SchedulePolicyResource{Name: "apps.v1.deployment.nginx"})
		policyTest := &v1alpha1.MultiClusterResourceSchedulePolicy{
			ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "schedule-test"},
			Spec: v1alpha1.MultiClusterResourceSchedulePolicySpec{Resources: resource,
				ClusterSource: v1alpha1.ClusterSourceTypeClusterset,
				Replicas:      20,
				// cluster1,2,3
				Clusterset: "cluster-set-selector",
				Reschedule: true,
				Policy:     policy,
			},
		}

		Expect(k8sClient.Create(ctx, policyTest)).Should(BeNil())
		policyNamespacedName := types.NamespacedName{
			Name:      policyTest.Name,
			Namespace: policyTest.Namespace,
		}
		// reconcile
		_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: policyNamespacedName})
		Expect(err).Should(BeNil())
		createdBinding := &v1alpha1.MultiClusterResourceBinding{}
		bindingName, _ := common.GenerateNameByOption(pkgcommon.Scheduler, policyTest.Name, "-")
		k8sClient.Get(context.TODO(), types.NamespacedName{Name: bindingName, Namespace: policyTest.Namespace}, createdBinding)
		Expect(createdBinding.Spec.Resources[0].Clusters[0].Name).Should(Equal("cluster1"))
		Expect(createdBinding.Spec.Resources[0].Clusters[1].Name).Should(Equal("cluster2"))
		Expect(len(createdBinding.Spec.Resources[0].Clusters)).Should(Equal(2))
		k8sClient.Delete(ctx, policyTest)
		k8sClient.Delete(ctx, createdBinding)
	})

})
