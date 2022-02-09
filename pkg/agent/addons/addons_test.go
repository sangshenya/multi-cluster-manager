package addons_test

import (
	"flag"
	"net"
	"strconv"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"harmonycloud.cn/stellaris/config"
	"harmonycloud.cn/stellaris/pkg/agent/addons"
	agentcfg "harmonycloud.cn/stellaris/pkg/agent/config"
	agentconfig "harmonycloud.cn/stellaris/pkg/agent/config"
	clientset "harmonycloud.cn/stellaris/pkg/client/clientset/versioned"
	corecfg "harmonycloud.cn/stellaris/pkg/core/config"
	"harmonycloud.cn/stellaris/pkg/core/handler"
	"harmonycloud.cn/stellaris/pkg/model"
	"harmonycloud.cn/stellaris/pkg/util/agent"
	"harmonycloud.cn/stellaris/pkg/util/common"
	"k8s.io/client-go/tools/clientcmd"
)

var _ = Describe("Addons", func() {
	var (
		inTree      []model.In
		outTree     []model.Out
		plugins     model.Plugins
		addonConfig model.PluginsConfig

		addonsInfoExcept []model.Addon
		requestExcept    *model.RegisterRequest

		lisPort = 8080
	)
	k8sconfig := flag.String("k8sconfig", "C:/Users/kuangye/Desktop/k8s/config", "kubernetes test")
	kubeCfg, _ := clientcmd.BuildConfigFromFlags("", *k8sconfig)

	cfg := agentcfg.DefaultConfiguration()
	cfg.HeartbeatPeriod = 30 * time.Second
	cfg.ClusterName = "cluster238"
	cfg.CoreAddress = ":8080"
	cfg.AddonPath = ""

	Describe("Addons starting", func() {
		Context("Load", func() {
			It("Get addons config", func() {
				in := model.In{Name: "addon1"}
				out := model.Out{Name: "addon2", Url: "www.123.com"}
				inTree = append(inTree, in)
				outTree = append(outTree, out)
				plugins = model.Plugins{InTree: inTree, OutTree: outTree}
				addonConfig = model.PluginsConfig{Plugins: plugins}

				res, _ := agent.GetAddonConfig("path/to/test.yaml")
				Expect(res).To(Equal(&addonConfig))
			})
			It("Load addons,will run thread for each plugin,and write data into a channel", func() {
				inTreeProperties := make(map[string]string)
				inTreeProperties["inTree"] = "addon1"
				inTreeRes := model.Addon{Name: "addon1", Properties: inTreeProperties}
				outTreeProperties := make(map[string]string)
				outTreeProperties["outTree"] = "www.123.com"
				outTreeRes := model.Addon{Name: "addon2", Properties: outTreeProperties}
				addonsInfoExcept = append(addonsInfoExcept, inTreeRes)
				addonsInfoExcept = append(addonsInfoExcept, outTreeRes)
				requestExcept = &model.RegisterRequest{Addons: addonsInfoExcept}

				//registerRequest, _ := addons.Load(&addonConfig)
				registerRequest := &model.RegisterRequest{}
				if agentconfig.AgentConfig.Cfg.AddonPath != "" {
					addonConfig, err := agent.GetAddonConfig(agentconfig.AgentConfig.Cfg.AddonPath)
					Expect(err).Should(BeNil())
					addonsList := addons.LoadAddon(addonConfig)
					registerRequest.Addons = addonsList
				}
				Expect(registerRequest).To(Equal(requestExcept))
			})
			It("Register", func() {
				// server
				addr := ":" + strconv.Itoa(lisPort)
				l, _ := net.Listen("tcp", addr)
				// construct client
				mClient, _ := clientset.NewForConfig(kubeCfg)
				serverConfig := corecfg.DefaultConfiguration()
				serverConfig.HeartbeatExpirePeriod = 30 * time.Second

				s := grpc.NewServer()
				config.RegisterChannelServer(s, &handler.Channel{
					Server: handler.NewCoreServer(serverConfig, mClient),
				})
				go func() {
					logrus.Infof("listening port %d", lisPort)
					err := s.Serve(l)
					Expect(err).Should(BeNil())
				}()

				// client
				stream, err := agent.Connection(cfg)
				Expect(err).Should(BeNil())
				addonInfo := &model.RegisterRequest{}
				request, _ := common.GenerateRequest("Register", addonInfo, cfg.ClusterName)
				err = stream.Send(request)
				Expect(err).Should(BeNil())
				resp, err := stream.Recv()
				Expect(err).Should(BeNil())
				respExpect := &config.Response{
					Type:        "RegisterSuccess",
					ClusterName: "cluster238",
				}
				Expect(resp.Type).Should(Equal(respExpect.Type))
				Expect(resp.ClusterName).Should(Equal(respExpect.ClusterName))

			})
			// TODO HEARTBEAT TEST
		})
	})
})
