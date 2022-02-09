package handler

import (
	"fmt"
	"io"

	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/sirupsen/logrus"
	"harmonycloud.cn/stellaris/config"
)

var coreServerLog = logf.Log.WithName("core_server")

type Fn func(request *config.Request, stream config.Channel_EstablishServer)

type Channel struct {
	Server *CoreServer
}

func (c *Channel) Establish(stream config.Channel_EstablishServer) error {
	clusterName := "(unknown)"

	for {
		req, err := stream.Recv()
		if err == io.EOF {
			break
		} else if err != nil {
			logrus.Error(err)
			break
		}
		if err := validate(req); err != nil {
			logrus.Error(err)
			continue
		}
		if clusterName == "(unknown)" {
			clusterName = req.ClusterName
		}
		for _, handler := range c.Server.Handlers[req.Type] {
			go handler(req, stream)
		}
	}

	coreServerLog.Info(fmt.Sprintf("connection with %s interrupt", clusterName))
	return nil
}

func validate(request *config.Request) error {
	if request.Type == "" {
		return fmt.Errorf("type field is empty in request")
	}
	if request.ClusterName == "" {
		return fmt.Errorf("clusterName field is empty in request")
	}
	return nil
}
