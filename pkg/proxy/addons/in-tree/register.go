package inTree

import (
	"context"
	"errors"
	"fmt"
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
	Prometheus        AddonRegisterType = "prometheus"
	Elasticsearch     AddonRegisterType = "elasticsearch"
	Ingress           AddonRegisterType = "ingress"
	ApiServer         AddonRegisterType = "kube-apiserver-healthy"
	ControllerManager AddonRegisterType = "kube-controller-manager-healthy"
	Scheduler         AddonRegisterType = "kube-scheduler-healthy"
	Etcd              AddonRegisterType = "kube-etcd-healthy"
	CoreDNS           AddonRegisterType = "coredns"
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
}

// load inTree plugins data
func LoadInTreeData(ctx context.Context, inTree *model.In) (*model.AddonsData, error) {
	loader, ok := AddonsRegisterMap[strings.ToLower(inTree.Name)]
	if !ok || loader == nil {
		return nil, errors.New(fmt.Sprintf("can not find inTree(%s)", inTree.Name))
	}
	return loader.Load(ctx, inTree)
}
