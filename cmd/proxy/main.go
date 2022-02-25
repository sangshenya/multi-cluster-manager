package main

import (
	"flag"
	"time"

	"harmonycloud.cn/stellaris/pkg/proxy/send"

	proxy_stream "harmonycloud.cn/stellaris/pkg/proxy/stream"

	"k8s.io/klog/v2"
	"k8s.io/klog/v2/klogr"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"harmonycloud.cn/stellaris/pkg/proxy/handler"

	"harmonycloud.cn/stellaris/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/healthz"

	"github.com/sirupsen/logrus"
	proxy_cfg "harmonycloud.cn/stellaris/pkg/proxy/config"
	"harmonycloud.cn/stellaris/pkg/apis/multicluster/v1alpha1"
	clientset "harmonycloud.cn/stellaris/pkg/client/clientset/versioned"
	controllerCommon "harmonycloud.cn/stellaris/pkg/controller/common"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
)

var (
	heartbeatPeriod  int
	coreAddress      string
	clusterName      string
	addonPath        string
	metricsAddr      string
	probeAddr        string
	addonLoadTimeout int
)

var proxyScheme = runtime.NewScheme()

func init() {
	flag.IntVar(&heartbeatPeriod, "heartbeat-send-period", 30, "The period of heartbeat send interval")
	// flag.StringVar(&coreAddress, "core-address", "10.1.11.46:32696", "address of stellaris")
	// flag.StringVar(&clusterName, "cluster-name", "example-test-1", "name of proxy-cluster")
	// flag.StringVar(&addonPath, "addon-path", "/Users/chenkun/Desktop/Go_Ad/src/harmonycloud.cn/stellaris/test.yaml", "path of addon config")
	flag.StringVar(&coreAddress, "core-address", "", "address of stellaris")
	flag.StringVar(&clusterName, "cluster-name", "", "name of proxy-cluster")
	flag.StringVar(&addonPath, "addon-path", "", "path of addon config")

	flag.StringVar(&metricsAddr, "metrics-addr", ":9000", "The address the metrics endpoint binds to")
	flag.StringVar(&probeAddr, "health-probe-addr", ":9001", "The address the probe endpoint binds to.")

	flag.IntVar(&addonLoadTimeout, "addon-load-timeout", 3, "Load addon timeout")
	utilruntime.Must(v1alpha1.AddToScheme(proxyScheme))
	utilruntime.Must(scheme.AddToScheme(proxyScheme))

}
func main() {

	flag.Parse()

	klog.InitFlags(nil)

	logf.SetLogger(klogr.New())

	cfg := proxy_cfg.DefaultConfiguration()
	cfg.HeartbeatPeriod = time.Duration(heartbeatPeriod) * time.Second
	cfg.ClusterName = clusterName
	cfg.CoreAddress = coreAddress
	cfg.AddonPath = addonPath
	cfg.AddonLoadTimeout = time.Duration(addonLoadTimeout) * time.Second

	restCfg := ctrl.GetConfigOrDie()
	mgr, err := ctrl.NewManager(restCfg, ctrl.Options{
		Scheme:                 proxyScheme,
		MetricsBindAddress:     metricsAddr,
		HealthProbeBindAddress: probeAddr,
	})
	if err != nil {
		logrus.Fatalf("failed create manager: %s", err)
	}

	controllerArgs := controllerCommon.Args{
		IsControlPlane: false,
	}
	// setup controllers
	if err = controller.Setup(mgr, controllerArgs); err != nil {
		logrus.Fatalf("failed to create controller: %s", err)
	}

	proxyClient, err := clientset.NewForConfig(restCfg)
	if err != nil {
		logrus.Fatalf("failed get proxyClient, clusterName:%s", cfg.ClusterName)
	}

	// new proxyConfig
	proxy_cfg.NewProxyConfig(cfg, proxyClient, mgr.GetClient())

	// new stream
	stream := proxy_stream.GetConnection()
	if stream == nil {
		logrus.Fatalf("failed get connection %s", cfg.CoreAddress)
	}

	err = send.Register()
	if err != nil {
		logrus.Fatalf("failed send register request, cluster: %s", err)
	}

	go handler.RecvResponse()

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		logrus.Fatalf("failed to setup health check")
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		logrus.Fatalf("failed to setup ready check")
	}
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		logrus.Fatalf("failed running manager: %s", err)
	}

}
