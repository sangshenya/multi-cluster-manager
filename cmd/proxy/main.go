package main

import (
	"flag"
	agentcfg "harmonycloud.cn/multi-cluster-manager/pkg/agent/config"
	"harmonycloud.cn/multi-cluster-manager/pkg/agent/handler"
	"time"
)

var (
	heartbeatPeriod time.Duration
	coreAddress     string
	clusterName     string
	addonPath       string
)

func init() {
	flag.DurationVar(&heartbeatPeriod, "heartbeat-send-period", 30, "The period of heartbeat send interval")
	flag.StringVar(&coreAddress, "core-address", "", "address of mcm-core")
	flag.StringVar(&clusterName, "cluster-name", "", "name of agent-cluster")
	flag.StringVar(&addonPath, "addon-path", "", "path of addon config")
}
func main() {
	flag.Parse()

	cfg := agentcfg.DefaultConfiguration()
	cfg.HeartbeatPeriod = heartbeatPeriod
	cfg.ClusterName = clusterName
	cfg.CoreAddress = coreAddress
	cfg.AddonPath = addonPath
	handler.Register(cfg)

}
