package stream

import (
	"context"
	"sync"
	"sync/atomic"

	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"google.golang.org/grpc/credentials/insecure"

	proxy_cfg "harmonycloud.cn/stellaris/pkg/proxy/config"

	"google.golang.org/grpc"
	"harmonycloud.cn/stellaris/config"
)

var initialized uint32
var mux sync.Mutex
var stream config.Channel_EstablishClient

var policyStreamLog = logf.Log.WithName("policy_stream")

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
		} else {
			policyStreamLog.Error(err, "Unable to get grpc connection")
		}
	}
	return stream
}

func getConnection() (config.Channel_EstablishClient, error) {
	conn, err := grpc.Dial(proxy_cfg.ProxyConfig.Cfg.CoreAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	grpcClient := config.NewChannelClient(conn)
	return grpcClient.Establish(context.Background())
}

func SetEmptyConnection() {
	atomic.StoreUint32(&initialized, 0)
	stream = nil
}
