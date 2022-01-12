package main

import (
	"flag"
	"time"

	"harmonycloud.cn/stellaris/pkg/agent/handler"

	"harmonycloud.cn/stellaris/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/healthz"

	"github.com/sirupsen/logrus"
	agentcfg "harmonycloud.cn/stellaris/pkg/agent/config"
	"harmonycloud.cn/stellaris/pkg/apis/multicluster/v1alpha1"
	clientset "harmonycloud.cn/stellaris/pkg/client/clientset/versioned"
	controllerCommon "harmonycloud.cn/stellaris/pkg/controller/common"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
)

var (
	heartbeatPeriod time.Duration
	coreAddress     string
	clusterName     string
	addonPath       string
	metricsAddr     string
	probeAddr       string
)

var scheme = runtime.NewScheme()

func init() {
	flag.DurationVar(&heartbeatPeriod, "heartbeat-send-period", 30, "The period of heartbeat send interval")
	flag.StringVar(&coreAddress, "core-address", "", "address of stellaris")
	flag.StringVar(&clusterName, "cluster-name", "", "name of agent-cluster")
	flag.StringVar(&addonPath, "addon-path", "", "path of addon config")

	flag.StringVar(&metricsAddr, "metrics-addr", ":9000", "The address the metrics endpoint binds to")
	flag.StringVar(&probeAddr, "health-probe-addr", ":9001", "The address the probe endpoint binds to.")

	utilruntime.Must(v1alpha1.AddToScheme(scheme))

}
func main() {
	flag.Parse()

	cfg := agentcfg.DefaultConfiguration()
	cfg.HeartbeatPeriod = heartbeatPeriod
	cfg.ClusterName = clusterName
	cfg.CoreAddress = coreAddress
	cfg.AddonPath = addonPath

	restCfg := ctrl.GetConfigOrDie()
	mgr, err := ctrl.NewManager(restCfg, ctrl.Options{
		Scheme:                 scheme,
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

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		logrus.Fatalf("failed to setup health check")
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		logrus.Fatalf("failed to setup ready check")
	}
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		logrus.Fatalf("failed running manager: %s", err)
	}

	agentClient, err := clientset.NewForConfig(restCfg)
	err = handler.Register(cfg, agentClient)
	if err != nil {
		logrus.Fatalf("failed register cluster: %s", err)
	}

}
