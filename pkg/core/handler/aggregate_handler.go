package handler

import (
	"fmt"

	"harmonycloud.cn/stellaris/config"
)

func (s *CoreServer) Aggregate(req *config.Request, stream config.Channel_EstablishServer) {
	resourceHandlerLog.Info(fmt.Sprintf("receive grpc request for aggregate, cluster:%s", req.ClusterName))
	// TODO receive grpc request for aggregate
}
