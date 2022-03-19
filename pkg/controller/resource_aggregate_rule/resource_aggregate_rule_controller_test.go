package resource_aggregate_rule

import (
	"context"
	"time"

	sliceutils "harmonycloud.cn/stellaris/pkg/utils/slice"

	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"k8s.io/apimachinery/pkg/types"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"harmonycloud.cn/stellaris/pkg/apis/multicluster/v1alpha1"
	managerCommon "harmonycloud.cn/stellaris/pkg/common"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Test MultiResourceAggregateRuleController", func() {
	// rule`s cue can not be empty
	// create MultiResourceAggregateRule,then validate Finalizer,check labels,check MultiResourceAggregateRule
	// update MultiResourceAggregateRule,then check MultiResourceAggregateRule
	// delete MultiResourceAggregateRule,then check MultiResourceAggregateRule
	var (
		rule           *v1alpha1.MultiClusterResourceAggregateRule
		gvk            *metav1.GroupVersionKind
		ruleName       string
		ctx            context.Context
		cue            string
		err            error
		ruleNamespaced types.NamespacedName
		ns             *v1.Namespace
	)
	BeforeEach(func() {

		ctx = context.Background()
		ruleName = "rule-test"

		ruleNamespaced = types.NamespacedName{
			Namespace: managerCommon.ManagerNamespace,
			Name:      ruleName,
		}

		gvk = &metav1.GroupVersionKind{
			Group:   "",
			Version: "v1",
			Kind:    "Endpoints",
		}

		rule = &v1alpha1.MultiClusterResourceAggregateRule{
			Spec: v1alpha1.MultiClusterResourceAggregateRuleSpec{
				ResourceRef: gvk,
				Rule: v1alpha1.AggregateRuleCue{
					Cue: cue,
				},
			},
		}
		rule.SetName(ruleName)
		rule.SetNamespace(managerCommon.ManagerNamespace)
		rule.SetGroupVersionKind(v1alpha1.MultiClusterResourceAggregateRuleGroupVersionKind)

		// create ns
		ns = &v1.Namespace{}
		ns.SetName(managerCommon.ManagerNamespace)
		err = k8sClient.Create(ctx, ns)
		if !apierrors.IsAlreadyExists(err) && err != nil {
			Expect(err).Should(BeNil())
		}
	})

	// create rule
	It("create rule when cue is not empty", func() {
		rule.Spec.Rule.Cue = endpointCue
		err = k8sClient.Create(ctx, rule)
		Expect(err).Should(BeNil())

		// Reconcile will add Finalizers
		_, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: ruleNamespaced})
		Expect(err).Should(BeNil())

		time.Sleep(3 * time.Second)

		err = k8sClient.Get(ctx, ruleNamespaced, rule)
		Expect(err).Should(BeNil())

		Expect(len(rule.GetFinalizers())).ShouldNot(Equal(0))
		Expect(sliceutils.ContainsString(rule.GetFinalizers(), managerCommon.FinalizerName)).Should(BeTrue())

		// Reconcile will add labels
		_, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: ruleNamespaced})
		Expect(err).Should(BeNil())

		time.Sleep(3 * time.Second)

		err = k8sClient.Get(ctx, ruleNamespaced, rule)
		Expect(err).Should(BeNil())

		Expect(len(rule.GetLabels())).ShouldNot(Equal(0))
		gvkString, ok := rule.Labels[managerCommon.AggregateResourceGvkLabelName]
		Expect(ok).Should(BeTrue())
		Expect(gvkString).Should(Equal(managerCommon.GvkLabelString(gvk)))

	})
	// update rule
	It("update rule", func() {
		err = k8sClient.Get(ctx, ruleNamespaced, rule)
		Expect(err).Should(BeNil())

		rule.Spec.Rule.Cue = endpointCue2
		err = k8sClient.Update(ctx, rule)
		Expect(err).Should(BeNil())

		time.Sleep(3 * time.Second)

		err = k8sClient.Get(ctx, ruleNamespaced, rule)
		Expect(err).Should(BeNil())

		Expect(rule.Spec.Rule.Cue).Should(Equal(endpointCue2))
	})

	// delete rule
	It("delete rule", func() {
		err = k8sClient.Delete(ctx, rule)
		Expect(err).Should(BeNil())

		// Reconcile will delete rule Finalizers
		_, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: ruleNamespaced})
		Expect(err).Should(BeNil())

		err = k8sClient.Get(ctx, ruleNamespaced, ns)
		Expect(apierrors.IsNotFound(err)).Should(BeTrue())
	})

	It("clear resource", func() {
		err = k8sClient.Get(ctx, types.NamespacedName{
			Name: managerCommon.ManagerNamespace,
		}, ns)
		Expect(err).Should(BeNil())

		err = k8sClient.Delete(ctx, ns)
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

const endpointCue2 = `
	output: endpoint: [
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
