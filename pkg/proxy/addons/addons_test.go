package addons_test

import (
	"flag"
	"time"

	"github.com/sirupsen/logrus"
	"harmonycloud.cn/stellaris/pkg/apis/multicluster/v1alpha1"
	clientset "harmonycloud.cn/stellaris/pkg/client/clientset/versioned"
	proxy_cfg "harmonycloud.cn/stellaris/pkg/proxy/config"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/kubernetes/scheme"
)

var cfg *rest.Config
var k8sClient client.Client
var testScheme = runtime.NewScheme()

var _ = Describe("Addons", func() {
	var ()

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

	proxyCfg := proxy_cfg.DefaultConfiguration()
	proxyCfg.HeartbeatPeriod = 30 * time.Second
	proxyCfg.ClusterName = "cluster238"
	proxyCfg.CoreAddress = ":8080"
	proxyCfg.AddonPath = "/Users/chenkun/Desktop/Go_Ad/src/harmonycloud.cn/stellaris/test.yaml"
	proxyCfg.AddonLoadTimeout = 3 * time.Second

	proxyClient, err := clientset.NewForConfig(cfg)
	if err != nil {
		logrus.Fatalf("failed get proxyClient, clusterName:%s", proxyCfg.ClusterName)
	}

	// new proxyConfig
	proxy_cfg.NewProxyConfig(proxyCfg, proxyClient, k8sClient, cfg)

	Describe("Addons starting", func() {
		Context("Load", func() {
			It("Get addons config", func() {

			})
		})
	})
})
