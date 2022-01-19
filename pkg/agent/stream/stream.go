package stream

import (
	"context"
	"sync"
	"sync/atomic"

	agentcfg "harmonycloud.cn/stellaris/pkg/agent/config"

	"google.golang.org/grpc"
	"harmonycloud.cn/stellaris/config"
)

var initialized uint32
var mux sync.Mutex
var stream config.Channel_EstablishClient

func GetConnection() config.Channel_EstablishClient {
	if atomic.LoadUint32(&initialized) == 1 {
		return stream
	}
	mux.Lock()
	defer mux.Unlock()
	if initialized == 0 {
		s, err := getConnection()
		if err == nil {
			stream = s
			atomic.StoreUint32(&initialized, 1)
		}
	}
	return nil
}

func getConnection() (config.Channel_EstablishClient, error) {
	conn, err := grpc.Dial(agentcfg.AgentConfig.Cfg.CoreAddress, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	grpcClient := config.NewChannelClient(conn)
	return grpcClient.Establish(context.Background())
}
