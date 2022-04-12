package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"harmonycloud.cn/stellaris/pkg/core/token"

	"harmonycloud.cn/stellaris/pkg/core/monitor"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"

	"k8s.io/klog/v2/klogr"

	"k8s.io/klog/v2"

	managerHelper "harmonycloud.cn/stellaris/pkg/common/helper"
	controllerCommon "harmonycloud.cn/stellaris/pkg/controller/common"
	managerWebhook "harmonycloud.cn/stellaris/pkg/webhook"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"harmonycloud.cn/stellaris/config"
	"harmonycloud.cn/stellaris/pkg/apis/multicluster/v1alpha1"
	clientset "harmonycloud.cn/stellaris/pkg/client/clientset/versioned"
	"harmonycloud.cn/stellaris/pkg/controller"
	corecfg "harmonycloud.cn/stellaris/pkg/core/config"
	"harmonycloud.cn/stellaris/pkg/core/handler"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var (
	coreScheme = runtime.NewScheme()
)

var (
	lisPort                  int
	heartbeatExpirePeriod    int
	onlineExpirationTime     int
	clusterStatusCheckPeriod int
	masterURL                string
	metricsAddr              string
	probeAddr                string
	certDir                  string
	tmplStr                  string
	webhookPort              int
)

func init() {
	flag.StringVar(&masterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
	flag.IntVar(&lisPort, "listen-port", 8080, "Bind port used to provider grpc serve")
	flag.IntVar(&heartbeatExpirePeriod, "heartbeat-expire-period", 30, "The period of maximum heartbeat interval")
	flag.IntVar(&clusterStatusCheckPeriod, "cluster-status-check-period", 60, "The period of check cluster status interval")
	flag.IntVar(&onlineExpirationTime, "online-expiration-time", 90, "cluster status online expiration time")
	flag.StringVar(&metricsAddr, "metrics-addr", ":9000", "The address the metrics endpoint binds to")
	flag.StringVar(&probeAddr, "health-probe-addr", ":9001", "The address the probe endpoint binds to.")
	flag.StringVar(&certDir, "webhook-cert-dir", "/k8s-webhook-server/serving-certs", "Admission webhook cert/key dir.")
	flag.IntVar(&webhookPort, "webhook-port", 9443, "Admission webhook listen address")

	utilruntime.Must(v1alpha1.AddToScheme(coreScheme))
	utilruntime.Must(scheme.AddToScheme(coreScheme))
}

func main() {

	flag.Parse()

	klog.InitFlags(nil)

	logf.SetLogger(klogr.New())

	addr := ":" + strconv.Itoa(lisPort)
	l, err := net.Listen("tcp", addr)
	if err != nil {
		logrus.Fatalf("listen port %d error: %s", lisPort, err)
	}

	namespace, err := managerHelper.GetOperatorNamespace()
	if err != nil {
		logrus.Fatalf("get namespace failed, error: %s", err)
	}

	// construct client
	kubeCfg, err := managerHelper.GetKubeConfig(masterURL)
	if err != nil {
		logrus.Fatalf("failed connect kube-apiserver: %s", err)
	}
	mClient, err := clientset.NewForConfig(kubeCfg)
	if err != nil {
		logrus.Fatalf("failed get multicluster client set: %s", err)
	}

	cfg := corecfg.DefaultConfiguration()
	cfg.HeartbeatExpirePeriod = time.Duration(heartbeatExpirePeriod) * time.Second
	cfg.ClusterStatusCheckPeriod = time.Duration(clusterStatusCheckPeriod) * time.Second
	cfg.OnlineExpirationTime = time.Duration(onlineExpirationTime) * time.Second

	restCfg := ctrl.GetConfigOrDie()
	mgr, err := ctrl.NewManager(restCfg, ctrl.Options{
		Scheme:                  coreScheme,
		MetricsBindAddress:      metricsAddr,
		CertDir:                 certDir,
		Port:                    webhookPort,
		HealthProbeBindAddress:  probeAddr,
		LeaderElectionNamespace: namespace,
		LeaderElection:          true,
		LeaderElectionID:        "stellaris-core-lock",
	})
	if err != nil {
		logrus.Fatalf("failed create manager: %s", err)
	}

	s := grpc.NewServer()
	config.RegisterChannelServer(s, &handler.Channel{
		Server: handler.NewCoreServer(cfg, mClient, mgr.GetClient()),
	})
	go func() {
		logrus.Infof("listening port %d", lisPort)
		if err = s.Serve(l); err != nil {
			logrus.Fatalf("grpc server running error: %s", err)
		}
	}()

	// get cue template configmap metadata
	tmplNs, tmplName, err := cache.SplitMetaNamespaceKey(tmplStr)
	if err != nil {
		logrus.Fatalf("--cue-template-config-map args must format be namespace/name, but got %s", tmplStr)
	}
	controllerArgs := controllerCommon.Args{
		IsControlPlane:     true,
		TmplNamespacedName: types.NamespacedName{Namespace: tmplNs, Name: tmplName},
	}

	// register webhook
	managerWebhook.Register(mgr, controllerArgs)
	if err = waitWebhookSecretVolume(certDir, 90*time.Second, 2*time.Second); err != nil {
		klog.ErrorS(err, "Unable to get webhook secret")
		os.Exit(1)
	}
	ctx := context.Background()
	// edit service
	if err = monitor.ConfigHeadlessService(ctx); err != nil {
		klog.ErrorS(err, "Unable to edit service")
		os.Exit(1)
	}
	// create token
	if err = token.CreateToken(ctx, mgr.GetClient()); err != nil {
		klog.ErrorS(err, "Unable to create token")
		os.Exit(1)
	}

	// monitor
	go monitor.StartCheckClusterStatus(mClient, cfg)

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
}

// waitWebhookSecretVolume waits for webhook secret ready to avoid mgr running crash
func waitWebhookSecretVolume(certDir string, timeout, interval time.Duration) error {
	start := time.Now()
	for {
		time.Sleep(interval)
		if time.Since(start) > timeout {
			return fmt.Errorf("getting webhook secret timeout after %s", timeout.String())
		}
		klog.InfoS("Wait webhook secret", "time consumed(second)", int64(time.Since(start).Seconds()),
			"timeout(second)", int64(timeout.Seconds()))
		if _, err := os.Stat(certDir); !os.IsNotExist(err) {
			ready := func() bool {
				f, err := os.Open(filepath.Clean(certDir))
				if err != nil {
					return false
				}
				// nolint
				defer f.Close()
				// check if dir is empty
				if _, err = f.Readdir(1); errors.Is(err, io.EOF) {
					return false
				}
				// check if secret files are empty
				err = filepath.Walk(certDir, func(path string, info os.FileInfo, err error) error {
					// even Cert dir is created, cert files are still empty for a while
					if info.Size() == 0 {
						return errors.New("secret is not ready")
					}
					return nil
				})
				if err == nil {
					klog.InfoS("Webhook secret is ready", "time consumed(second)",
						int64(time.Since(start).Seconds()))
					return true
				}
				return false
			}()
			if ready {
				return nil
			}
		}
	}
}
