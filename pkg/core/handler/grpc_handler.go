package handler

import (
	multclusterclient "harmonycloud.cn/stellaris/pkg/client/clientset/versioned"
	corecfg "harmonycloud.cn/stellaris/pkg/core/config"
	"harmonycloud.cn/stellaris/pkg/model"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type CoreServer struct {
	Handlers  map[string][]Fn
	Config    *corecfg.Configuration
	mClient   *multclusterclient.Clientset
	clientSet client.Client
}

func NewCoreServer(cfg *corecfg.Configuration, mClient *multclusterclient.Clientset, clientSet client.Client) *CoreServer {
	s := &CoreServer{Config: cfg}
	s.mClient = mClient
	s.clientSet = clientSet
	s.init()
	return s
}

func (s *CoreServer) init() {
	s.Handlers = make(map[string][]Fn)
	s.registerHandler(model.Register.String(), s.Register)
	s.registerHandler(model.Heartbeat.String(), s.Heartbeat)
	s.registerHandler(model.Resource.String(), s.Resource)
	s.registerHandler(model.Aggregate.String(), s.Aggregate)
}

func (s *CoreServer) registerHandler(typ string, fn Fn) {
	fns := s.Handlers[typ]
	if fns == nil {
		fns = make([]Fn, 0, 5)
	}
	fns = append(fns, fn)
	s.Handlers[typ] = fns
}
