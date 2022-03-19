package multi_resource_aggregate_policy

import (
	"context"
	"reflect"
	"time"

	"harmonycloud.cn/stellaris/pkg/apis/multicluster/common"

	sliceutils "harmonycloud.cn/stellaris/pkg/utils/slice"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	. "github.com/onsi/gomega"

	managerCommon "harmonycloud.cn/stellaris/pkg/common"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	. "github.com/onsi/ginkgo"
	"harmonycloud.cn/stellaris/pkg/apis/multicluster/v1alpha1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("Test MultiResourceAggregatePolicyController", func() {
	// create MultiResourceAggregatePolicy,then validate Finalizer,check labels,check ResourceAggregatePolicy
	// update MultiResourceAggregatePolicy,then check ResourceAggregatePolicy
	// delete MultiResourceAggregatePolicy,then check ResourceAggregatePolicy

	var (
		mPolicy           *v1alpha1.MultiClusterResourceAggregatePolicy
		rule              *v1alpha1.MultiClusterResourceAggregateRule
		gvk               *metav1.GroupVersionKind
		mPolicyName       string
		ruleName          string
		clusterName       string
		ctx               context.Context
		err               error
		ruleNamespaced    types.NamespacedName
		mPolicynamespaced types.NamespacedName
		ns                *v1.Namespace
		clusterNs         *v1.Namespace
	)
	It("create rule", func() {

		ctx = context.Background()
		ruleName = "rule-test"
		mPolicyName = "mpolicy-test"
		clusterName = "cluster1"

		ruleNamespaced = types.NamespacedName{
			Namespace: managerCommon.ManagerNamespace,
			Name:      ruleName,
		}
		mPolicynamespaced = types.NamespacedName{
			Namespace: managerCommon.ManagerNamespace,
			Name:      mPolicyName,
		}

		gvk = &metav1.GroupVersionKind{
			Group:   "",
			Version: "v1",
			Kind:    "Endpoints",
		}
		// rule
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
		mPolicy = &v1alpha1.MultiClusterResourceAggregatePolicy{
			Spec: v1alpha1.MultiClusterResourceAggregatePolicySpec{
				AggregateRules: []string{
					ruleName,
				},
				Policy: v1alpha1.AggregatePolicySameNsMappingName,
				Limit: &v1alpha1.AggregatePolicyLimit{
					Requests: &v1alpha1.AggregatePolicyLimitRule{
						Match: []v1alpha1.Match{
							{
								Namespaces: "harbor-system",
								NameMatch: &v1alpha1.MatchScope{
									List: []string{
										"harborv2-ha-harbor-core",
									},
								},
							},
						},
					},
				},
			},
		}
		mPolicy.Spec.Clusters = &v1alpha1.AggregatePolicyClusters{
			ClusterType: common.ClusterTypeClusters,
			Clusters: []string{
				"cluster2",
			},
		}
		mPolicy.SetName(mPolicyName)
		mPolicy.SetNamespace(managerCommon.ManagerNamespace)
		mPolicy.SetGroupVersionKind(v1alpha1.MultiClusterResourceAggregatePolicyGroupVersionKind)

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

		// create rule
		err = k8sClient.Create(ctx, rule)
		Expect(err).Should(BeNil())
	})

	// create
	It("create mPolicy", func() {
		err = k8sClient.Create(ctx, mPolicy)
		Expect(err).Should(BeNil())

		_, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: mPolicynamespaced})
		Expect(err).Should(BeNil())

		time.Sleep(5 * time.Second)

		err = k8sClient.Get(ctx, mPolicynamespaced, mPolicy)
		Expect(err).Should(BeNil())

		// check Finalizers
		Expect(len(mPolicy.GetFinalizers())).ShouldNot(Equal(0))
		Expect(sliceutils.ContainsString(mPolicy.GetFinalizers(), managerCommon.FinalizerName)).Should(BeTrue())

		// Reconcile will add labels
		_, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: mPolicynamespaced})
		Expect(err).Should(BeNil())

		time.Sleep(3 * time.Second)

		err = k8sClient.Get(ctx, mPolicynamespaced, mPolicy)
		Expect(err).Should(BeNil())

		Expect(len(mPolicy.GetLabels())).ShouldNot(Equal(0))
		value, ok := mPolicy.Labels[managerCommon.AggregateRuleLabelName+"."+ruleName]
		Expect(ok).Should(BeTrue())
		Expect(value).Should(Equal("1"))
	})

	// update
	It("update mPolicy", func() {
		err = k8sClient.Get(ctx, mPolicynamespaced, mPolicy)
		Expect(err).Should(BeNil())

		mPolicy.Spec.Clusters.Clusters = []string{
			clusterName,
		}

		err = k8sClient.Update(ctx, mPolicy)
		Expect(err).Should(BeNil())

		time.Sleep(3 * time.Second)

		// Reconcile will create policy in clusterNs
		_, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: mPolicynamespaced})
		Expect(err).Should(BeNil())

		time.Sleep(3 * time.Second)

		// check policy
		policyName := resourceAggregatePolicyName(rule.Spec.ResourceRef)
		policy := &v1alpha1.ResourceAggregatePolicy{}
		err = k8sClient.Get(ctx, types.NamespacedName{
			Namespace: managerCommon.ClusterNamespace(clusterName),
			Name:      policyName,
		}, policy)
		Expect(err).Should(BeNil())

		Expect(reflect.DeepEqual(policy.Spec.ResourceRef, gvk)).Should(BeTrue())
		Expect(reflect.DeepEqual(policy.Spec.Limit, mPolicy.Spec.Limit)).Should(BeTrue())

	})

	// delete
	It("delete mPolicy", func() {
		err = k8sClient.Get(ctx, mPolicynamespaced, mPolicy)
		Expect(err).Should(BeNil())

		err = k8sClient.Delete(ctx, mPolicy)
		Expect(err).Should(BeNil())

		// Reconcile will delete Finalizers
		_, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: mPolicynamespaced})
		Expect(err).Should(BeNil())

		time.Sleep(10 * time.Second)

		err = k8sClient.Get(ctx, mPolicynamespaced, mPolicy)
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
