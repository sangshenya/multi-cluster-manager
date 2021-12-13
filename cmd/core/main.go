package main

import (
	"flag"
	"net"
	"strconv"
	"time"

	controllerCommon "harmonycloud.cn/stellaris/pkg/controller/common"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"harmonycloud.cn/stellaris/config"
	"harmonycloud.cn/stellaris/pkg/apis/multicluster/v1alpha1"
	clientset "harmonycloud.cn/stellaris/pkg/client/clientset/versioned"
	"harmonycloud.cn/stellaris/pkg/controller"
	corecfg "harmonycloud.cn/stellaris/pkg/core/config"
	"harmonycloud.cn/stellaris/pkg/core/handler"
	"harmonycloud.cn/stellaris/pkg/core/utils"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
)

var (
	scheme = runtime.NewScheme()
)

var (
	lisPort               int
	heartbeatExpirePeriod time.Duration
	masterURL             string
	metricsAddr           string
	probeAddr             string
)

func init() {
	flag.StringVar(&masterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
	flag.IntVar(&lisPort, "listen-port", 8080, "Bind port used to provider grpc serve")
	flag.DurationVar(&heartbeatExpirePeriod, "heartbeat-expire-period", 30, "The period of maximum heartbeat interval")
	flag.StringVar(&metricsAddr, "metrics-addr", ":9000", "The address the metrics endpoint binds to")
	flag.StringVar(&probeAddr, "health-probe-addr", ":9001", "The address the probe endpoint binds to.")

	utilruntime.Must(v1alpha1.AddToScheme(scheme))
}

func main() {
	flag.Parse()

	addr := ":" + strconv.Itoa(lisPort)
	l, err := net.Listen("tcp", addr)
	if err != nil {
		logrus.Fatalf("listen port %d error: %s", lisPort, err)
	}

	// construct client
	kubeCfg, err := utils.GetKubeConfig(masterURL)
	if err != nil {
		logrus.Fatalf("failed connect kube-apiserver: %s", err)
	}
	mClient, err := clientset.NewForConfig(kubeCfg)
	if err != nil {
		logrus.Fatalf("failed get multicluster client set: %s", err)
	}

	cfg := corecfg.DefaultConfiguration()
	cfg.HeartbeatExpirePeriod = heartbeatExpirePeriod

	s := grpc.NewServer()
	config.RegisterChannelServer(s, &handler.Channel{
		Server: handler.NewCoreServer(cfg, mClient),
	})
	go func() {
		logrus.Infof("listening port %d", lisPort)
		if err := s.Serve(l); err != nil {
			logrus.Fatalf("grpc server running error: %s", err)
		}
	}()

	restCfg := ctrl.GetConfigOrDie()
	mgr, err := ctrl.NewManager(restCfg, ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		Port:                   9443,
		HealthProbeBindAddress: probeAddr,
	})
	if err != nil {
		logrus.Fatalf("failed create manager: %s", err)
	}

	// setup controllers
	controllerArgs := controllerCommon.Args{ManagerClientSet: mClient}
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
}
