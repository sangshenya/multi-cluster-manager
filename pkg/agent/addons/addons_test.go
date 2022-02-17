package addons_test

import (
	"encoding/json"
	"flag"
	"fmt"
	"reflect"
	"time"

	"github.com/sirupsen/logrus"
	agentcfg "harmonycloud.cn/stellaris/pkg/agent/config"
	"harmonycloud.cn/stellaris/pkg/apis/multicluster/v1alpha1"
	clientset "harmonycloud.cn/stellaris/pkg/client/clientset/versioned"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"harmonycloud.cn/stellaris/pkg/agent/addons"
	"harmonycloud.cn/stellaris/pkg/model"
	"harmonycloud.cn/stellaris/pkg/util/agent"
	"k8s.io/client-go/kubernetes/scheme"
)

var cfg *rest.Config
var k8sClient client.Client
var testScheme = runtime.NewScheme()

var _ = Describe("Addons", func() {
	var (
		in          []model.In
		out         []model.Out
		plugins     model.Plugins
		addonConfig model.PluginsConfig
	)

	k8sconfig := flag.String("k8sconfig", "/Users/chenkun/Desktop/k8s/config-238", "kubernetes auth config")
	config, _ := clientcmd.BuildConfigFromFlags("", *k8sconfig)
	cfg = config

	err := v1alpha1.SchemeBuilder.AddToScheme(testScheme)
	Expect(err).NotTo(HaveOccurred())

	err = scheme.AddToScheme(testScheme)
	Expect(err).NotTo(HaveOccurred())

	k8sClient, err = client.New(cfg, client.Options{Scheme: testScheme})
	Expect(err).ToNot(HaveOccurred())
	Expect(k8sClient).ToNot(BeNil())

	agentCfg := agentcfg.DefaultConfiguration()
	agentCfg.HeartbeatPeriod = 30 * time.Second
	agentCfg.ClusterName = "cluster238"
	agentCfg.CoreAddress = ":8080"
	agentCfg.AddonPath = "/Users/chenkun/Desktop/Go_Ad/src/harmonycloud.cn/stellaris/test.yaml"
	agentCfg.AddonLoadTimeout = 3 * time.Second

	agentClient, err := clientset.NewForConfig(cfg)
	if err != nil {
		logrus.Fatalf("failed get agentClient, clusterName:%s", agentCfg.ClusterName)
	}

	// new agentConfig
	agentcfg.NewAgentConfig(agentCfg, agentClient, k8sClient)

	Describe("Addons starting", func() {
		Context("Load", func() {
			It("Get addons config", func() {
				in1 := model.In{Name: "apiserver"}
				in2 := model.In{Name: "etcd"}
				out1 := model.Out{Name: "test", Url: "http://47.97.243.214/goad/health"}
				in = append(in, in2, in1)
				out = append(out, out1)
				plugins = model.Plugins{InTree: in, OutTree: out}
				addonConfig = model.PluginsConfig{Plugins: plugins}

				res, err := agent.GetAddonConfig(agentCfg.AddonPath)
				Expect(err).Should(BeNil())
				Expect(reflect.DeepEqual(*res, addonConfig)).Should(BeTrue())

				addonsList1 := addons.LoadAddon(res)
				logJson(addonsList1)
				addonsList2 := addons.LoadAddon(&addonConfig)
				logJson(addonsList2)
				Expect(len(addonsList1)).Should(Equal(len(addonsList2)))
				for _, item1 := range addonsList1 {
					for _, item2 := range addonsList2 {
						if item1.Name == item2.Name {
							fmt.Println(item1, item2)
							Expect(AddonPropertiesEqual(item1, item2)).Should(BeTrue())
						}
					}
				}
			})
		})
	})
})

func AddonPropertiesEqual(addon1, addon2 model.Addon) bool {
	if addon1.Name != addon2.Name {
		return false
	}
	data1 := marshal(addon1.Properties)
	if data1 == nil {
		return false
	}
	data2 := marshal(addon2.Properties)
	if data2 == nil {
		return false
	}
	return reflect.DeepEqual(data1.Data, data2.Data)
}

func marshal(p interface{}) *model.PluginsData {
	ma, err := json.Marshal(p)
	if err != nil {
		return nil
	}
	b := &model.PluginsData{}
	err = json.Unmarshal(ma, b)
	if err != nil {
		return nil
	}
	return b
}

func logJson(v interface{}) {
	ma, err := json.Marshal(v)
	fmt.Println(string(ma), err)
}
