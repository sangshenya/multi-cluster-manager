package inTree

import (
	"context"
	"errors"
	"strings"

	"harmonycloud.cn/stellaris/pkg/model"
)

var (
	AddonsRegisterMap map[string]addonsLoader
)

type addonsLoader interface {
	Load(ctx context.Context, inTree *model.In) (*model.AddonsData, error)
}

type AddonRegisterType string

const (
	//Prometheus        AddonRegisterType = "prometheus"
	//Elasticsearch     AddonRegisterType = "elasticsearch"
	//Ingress           AddonRegisterType = "ingress"
	ApiServer         AddonRegisterType = "kube-apiserver-healthy"
	ControllerManager AddonRegisterType = "kube-controller-manager-healthy"
	Scheduler         AddonRegisterType = "kube-scheduler-healthy"
	Etcd              AddonRegisterType = "kube-etcd-healthy"
	CoreDNS           AddonRegisterType = "coredns"
	Calico            AddonRegisterType = "calico"
	Logging           AddonRegisterType = "logging"
	problemIsolation  AddonRegisterType = "problem-isolation"
)

func (a AddonRegisterType) String() string {
	return string(a)
}

func init() {
	AddonsRegisterMap = map[string]addonsLoader{}
	// register inTree addons
	AddonsRegisterMap[ApiServer.String()] = &kubeAddons{}
	AddonsRegisterMap[Etcd.String()] = &kubeAddons{}
	AddonsRegisterMap[ControllerManager.String()] = &kubeAddons{}
	AddonsRegisterMap[Scheduler.String()] = &kubeAddons{}
	AddonsRegisterMap[CoreDNS.String()] = &coreDNSAddons{}
	AddonsRegisterMap[Calico.String()] = &kubeAddons{}
	AddonsRegisterMap[Logging.String()] = &loggingAddons{}
	AddonsRegisterMap[problemIsolation.String()] = &kubeAddons{}
}

// LoadInTreeData load inTree addon data
func LoadInTreeData(ctx context.Context, inTree *model.In) (*model.AddonsData, error) {
	loader, ok := AddonsRegisterMap[strings.ToLower(inTree.Name)]
	if !ok || loader == nil {
		return nil, errors.New("can not find inTree" + inTree.Name)
	}
	return loader.Load(ctx, inTree)
}
