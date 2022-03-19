package resource_aggregate_policy

import (
	"context"
	"time"

	"harmonycloud.cn/stellaris/pkg/proxy/aggregate"

	sliceutils "harmonycloud.cn/stellaris/pkg/utils/slice"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	managerCommon "harmonycloud.cn/stellaris/pkg/common"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"harmonycloud.cn/stellaris/pkg/apis/multicluster/v1alpha1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("Test ResourceAggregatePolicyController", func() {
	var (
		policy           *v1alpha1.ResourceAggregatePolicy
		rule             *v1alpha1.MultiClusterResourceAggregateRule
		gvk              *metav1.GroupVersionKind
		policyName       string
		ruleName         string
		clusterName      string
		ctx              context.Context
		err              error
		ruleNamespaced   types.NamespacedName
		policynamespaced types.NamespacedName
		ns               *v1.Namespace
		clusterNs        *v1.Namespace
	)

	ctx = context.Background()
	It("create rule", func() {
		ruleName = "rule-test"
		policyName = "policy-test"
		clusterName = "cluster1"

		ruleNamespaced = types.NamespacedName{
			Namespace: managerCommon.ManagerNamespace,
			Name:      ruleName,
		}
		policynamespaced = types.NamespacedName{
			Namespace: managerCommon.ClusterNamespace(clusterName),
			Name:      policyName,
		}
		if !reconciler.isControlPlane {
			policynamespaced.Namespace = managerCommon.ManagerNamespace
		}

		gvk = &metav1.GroupVersionKind{
			Group:   "",
			Version: "v1",
			Kind:    "Endpoints",
		}
		//rule
		rule = &v1alpha1.MultiClusterResourceAggregateRule{
			Spec: v1alpha1.MultiClusterResourceAggregateRuleSpec{
				ResourceRef: gvk,
				Rule: v1alpha1.AggregateRuleCue{
					Cue: endpointCue,
				},
			},
		}
		rule.SetName(ruleName)
		rule.SetNamespace(managerCommon.ManagerNamespace)
		rule.SetGroupVersionKind(v1alpha1.MultiClusterResourceAggregateRuleGroupVersionKind)

		// mPolicy
		policy = &v1alpha1.ResourceAggregatePolicy{
			Spec: v1alpha1.ResourceAggregatePolicySpec{
				ResourceRef: gvk,
				Limit: &v1alpha1.AggregatePolicyLimit{
					Requests: &v1alpha1.AggregatePolicyLimitRule{
						Match: []v1alpha1.Match{
							{
								Namespaces: "harbor-system",
								NameMatch: &v1alpha1.MatchScope{
									List: []string{
										"core",
									},
								},
							},
						},
					},
				},
			},
		}
		policy.SetName(policyName)
		policy.SetNamespace(managerCommon.ClusterNamespace(clusterName))
		if !reconciler.isControlPlane {
			policy.SetNamespace(managerCommon.ManagerNamespace)
		}
		policy.SetGroupVersionKind(v1alpha1.MultiClusterResourceAggregatePolicyGroupVersionKind)

		labels := map[string]string{}
		labels[managerCommon.AggregateRuleLabelName] = ruleName
		labels[managerCommon.ParentResourceNamespaceLabelName] = managerCommon.ClusterNamespace(clusterName)
		// create ns
		ns = &v1.Namespace{}
		ns.SetName(managerCommon.ManagerNamespace)
		err = k8sClient.Create(ctx, ns)
		if !apierrors.IsAlreadyExists(err) && err != nil {
			Expect(err).Should(BeNil())
		}

		// create cluster workspace
		clusterNs = &v1.Namespace{}
		clusterNs.SetName(managerCommon.ClusterNamespace(clusterName))
		err = k8sClient.Create(ctx, clusterNs)
		if !apierrors.IsAlreadyExists(err) && err != nil {
			Expect(err).Should(BeNil())
		}

		if !reconciler.isControlPlane {
			// create rule
			err = k8sClient.Create(ctx, rule)
			Expect(err).Should(BeNil())
		}

	})

	// create policy
	It("create Policy", func() {
		err = k8sClient.Create(ctx, policy)
		Expect(err).Should(BeNil())

		_, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: policynamespaced})
		Expect(err).Should(BeNil())

		time.Sleep(5 * time.Second)

		err = k8sClient.Get(ctx, policynamespaced, policy)
		Expect(err).Should(BeNil())

		// check Finalizers
		Expect(len(policy.GetFinalizers())).ShouldNot(Equal(0))
		Expect(sliceutils.ContainsString(policy.GetFinalizers(), managerCommon.FinalizerName)).Should(BeTrue())
	})

	// update policy
	It("update policy", func() {
		err = k8sClient.Get(ctx, policynamespaced, policy)
		Expect(err).Should(BeNil())

		policy.Spec.Limit.Requests.Match = []v1alpha1.Match{
			{
				Namespaces: "harbor-system",
				NameMatch: &v1alpha1.MatchScope{
					List: []string{
						"harborv2-ha-harbor-core-1",
					},
				},
			},
		}

		err = k8sClient.Update(ctx, policy)
		Expect(err).Should(BeNil())

		time.Sleep(5 * time.Second)

		_, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: policynamespaced})
		Expect(err).Should(BeNil())

		time.Sleep(5 * time.Second)

		err = k8sClient.Get(ctx, policynamespaced, policy)
		Expect(err).Should(BeNil())

		match := policy.Spec.Limit.Requests.Match
		Expect(len(match)).Should(Equal(1))
		Expect(match[0].NameMatch.List[0]).Should(Equal("harborv2-ha-harbor-core-1"))

		specList := aggregate.GetInformerResourceConfig(gvk)
		Expect(len(specList)).Should(Equal(1))

		matchs := specList[0].Limit.Requests.Match
		Expect(len(matchs)).Should(Equal(1))
		Expect(matchs[0].NameMatch.List[0]).Should(Equal("harborv2-ha-harbor-core-1"))

	})

	// delete policy
	It("delete policy", func() {
		err = k8sClient.Get(ctx, policynamespaced, policy)
		Expect(err).Should(BeNil())

		err = k8sClient.Delete(ctx, policy)
		Expect(err).Should(BeNil())

		time.Sleep(8 * time.Second)
		// Reconcile will delete Finalizers
		_, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: policynamespaced})
		Expect(err).Should(BeNil())

		time.Sleep(10 * time.Second)

		err = k8sClient.Get(ctx, policynamespaced, policy)
		Expect(apierrors.IsNotFound(err)).Should(BeTrue())
	})

	It("clear resource", func() {
		err = k8sClient.Get(ctx, ruleNamespaced, rule)
		Expect(err).Should(BeNil())

		err = k8sClient.Delete(ctx, rule)
		Expect(err).Should(BeNil())

		err = k8sClient.Delete(ctx, ns)
		Expect(err).Should(BeNil())

		err = k8sClient.Delete(ctx, clusterNs)
		Expect(err).Should(BeNil())
	})

})

const endpointCue = `
	output: endpoints: [
		for _, v in {
			if context.subsets != _|_ && len(context.subsets) > 0 {
				for subset in context.subsets {
					if subset.addresses != _|_ && len(subset.addresses) > 0 && subset.ports != _|_ && len(subset.ports) > 0 {
						for i, addr in subset.addresses {
							for j, po in subset.ports {
								{
									"\(i)-\(j)": addr.ip + ":\(po.port)"
								},
							}
						}
					}
				}
			}
		} {v}
	]
`
