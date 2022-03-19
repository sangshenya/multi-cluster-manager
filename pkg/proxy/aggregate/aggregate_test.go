package aggregate

import (
	"context"
	"flag"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"k8s.io/apimachinery/pkg/types"

	"harmonycloud.cn/stellaris/pkg/proxy/aggregate/match"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	clientset "harmonycloud.cn/stellaris/pkg/client/clientset/versioned"
	proxy_cfg "harmonycloud.cn/stellaris/pkg/proxy/config"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"harmonycloud.cn/stellaris/pkg/apis/multicluster/v1alpha1"
	multclusterclient "harmonycloud.cn/stellaris/pkg/client/clientset/versioned"
	managerCommon "harmonycloud.cn/stellaris/pkg/common"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Aggregate", func() {
	var (
		cfg        *rest.Config
		k8sClient  client.Client
		testScheme = runtime.NewScheme()
	)
	ctx := context.Background()
	k8sconfig := flag.String("k8sconfig", "/Users/chenkun/Desktop/k8s/config-205", "kubernetes test")
	cfg, _ = clientcmd.BuildConfigFromFlags("", *k8sconfig)

	err := v1alpha1.SchemeBuilder.AddToScheme(testScheme)
	Expect(err).NotTo(HaveOccurred())

	err = scheme.AddToScheme(testScheme)
	Expect(err).NotTo(HaveOccurred())

	k8sClient, err = client.New(cfg, client.Options{Scheme: testScheme})
	Expect(err).ToNot(HaveOccurred())
	Expect(k8sClient).ToNot(BeNil())

	proxyClient, err := clientset.NewForConfig(cfg)
	Expect(err).Should(BeNil())

	// new proxyConfig
	c := proxy_cfg.DefaultConfiguration()
	c.ClusterName = "cluster205"
	proxy_cfg.NewProxyConfig(c, proxyClient, k8sClient, cfg)

	ns := &v1.Namespace{}
	ns.Name = managerCommon.ManagerNamespace
	err = k8sClient.Create(ctx, ns)
	if !apierrors.IsAlreadyExists(err) && err != nil {
		Expect(err).Should(BeNil())
	}

	// create ResourceAggregatePolicy and MultiClusterResourceAggregateRule
	dealCoreResponse(ctx, proxyClient)

	It("match", func() {
		// get ResourceAggregatePolicy
		policy, err := proxyClient.MulticlusterV1alpha1().ResourceAggregatePolicies(managerCommon.ManagerNamespace).Get(ctx, "policy-test", metav1.GetOptions{})
		Expect(err).Should(BeNil())

		coreEp := &unstructured.Unstructured{}
		coreEp.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   "",
			Version: "v1",
			Kind:    "Endpoints",
		})
		err = k8sClient.Get(ctx, types.NamespacedName{
			Name:      "harborv2-ha-harbor-core",
			Namespace: "harbor-system",
		}, coreEp)

		isTargetResource := match.IsTargetResourceWithConfig(ctx, coreEp, &policy.Spec)
		Expect(isTargetResource).Should(BeTrue())
	})

	It("Test proxy aggregate and informer resource", func() {
		// get ResourceAggregatePolicy
		policy, err := proxyClient.MulticlusterV1alpha1().ResourceAggregatePolicies(managerCommon.ManagerNamespace).Get(ctx, "policy-test", metav1.GetOptions{})
		Expect(err).Should(BeNil())
		err = AddInformerResourceConfig(policy)
		Expect(err).Should(BeNil())

		time.Sleep(40 * time.Second)

		err = RemoveInformerResourceConfig(policy)
		Expect(err).Should(BeNil())
	})

	It("delete resource", func() {
		deleteResource(ctx, proxyClient)

		err = k8sClient.Get(ctx, types.NamespacedName{
			Name: managerCommon.ManagerNamespace,
		}, ns)
		Expect(err).Should(BeNil())

		err = k8sClient.Delete(ctx, ns)
		Expect(err).Should(BeNil())
	})
})

func createEp(ctx context.Context, k8sClient client.Client) {
	ep := &v1.Endpoints{}
	ep.SetName("ep-test")
	ep.SetNamespace("default")
	err := k8sClient.Create(ctx, ep)
	Expect(err).Should(BeNil())
}

func deleteEp(ctx context.Context, k8sClient client.Client) {
	ep := &v1.Endpoints{}
	ep.SetName("ep-test")
	ep.SetNamespace("default")
	err := k8sClient.Delete(ctx, ep)
	Expect(err).Should(BeNil())
}

func deleteResource(ctx context.Context, proxyClient *multclusterclient.Clientset) {
	err := proxyClient.MulticlusterV1alpha1().ResourceAggregatePolicies(managerCommon.ManagerNamespace).Delete(ctx, "policy-test", metav1.DeleteOptions{})
	Expect(err).Should(BeNil())

	err = proxyClient.MulticlusterV1alpha1().MultiClusterResourceAggregateRules(managerCommon.ManagerNamespace).Delete(ctx, "ep-rule", metav1.DeleteOptions{})
	Expect(err).Should(BeNil())
}

// create ResourceAggregatePolicy and MultiClusterResourceAggregateRule
func dealCoreResponse(ctx context.Context, proxyClient *multclusterclient.Clientset) {
	gvk := &metav1.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "Endpoints",
	}
	policy := &v1alpha1.ResourceAggregatePolicy{}
	policy.SetName("policy-test")
	policy.SetNamespace(managerCommon.ManagerNamespace)
	policy.SetGroupVersionKind(v1alpha1.ResourceAggregatePolicyGroupVersionKind)
	labels := make(map[string]string, 1)
	labels[managerCommon.AggregateRuleLabelName] = managerCommon.ManagerNamespace + "." + "ep-rule"
	policy.SetLabels(labels)
	policy.Spec = v1alpha1.ResourceAggregatePolicySpec{
		ResourceRef: gvk,
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
	}

	rule := &v1alpha1.MultiClusterResourceAggregateRule{}
	rule.SetName("ep-rule")
	rule.SetNamespace(managerCommon.ManagerNamespace)
	rule.SetGroupVersionKind(v1alpha1.MultiClusterResourceAggregateRuleGroupVersionKind)

	ruleLabels := make(map[string]string, 1)
	targetGvkString := managerCommon.GvkLabelString(gvk)
	ruleLabels[managerCommon.AggregateResourceGvkLabelName] = targetGvkString
	rule.SetLabels(ruleLabels)

	rule.Spec = v1alpha1.MultiClusterResourceAggregateRuleSpec{
		ResourceRef: gvk,
		Rule: v1alpha1.AggregateRuleCue{
			Cue: endpointCue,
		},
	}

	var err error
	policy, err = proxyClient.MulticlusterV1alpha1().ResourceAggregatePolicies(policy.Namespace).Create(ctx, policy, metav1.CreateOptions{})
	Expect(err).Should(BeNil())

	rule, err = proxyClient.MulticlusterV1alpha1().MultiClusterResourceAggregateRules(rule.GetNamespace()).Create(ctx, rule, metav1.CreateOptions{})
	Expect(err).Should(BeNil())
}

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
