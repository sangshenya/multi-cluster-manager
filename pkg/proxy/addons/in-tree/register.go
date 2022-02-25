package inTree

import (
	"errors"
	"fmt"
	"strings"

	"harmonycloud.cn/stellaris/pkg/model"
)

var (
	AddonsRegisterMap map[string]addonsLoader
)

type addonsLoader interface {
	Load() (*model.PluginsData, error)
}

type AddonRegisterType string

const (
	Prometheus    AddonRegisterType = "prometheus"
	Elasticsearch AddonRegisterType = "elasticsearch"
	Ingress       AddonRegisterType = "ingress"
	ApiServer     AddonRegisterType = "apiserver"
	Etcd          AddonRegisterType = "etcd"
)

func (a AddonRegisterType) String() string {
	return string(a)
}

func init() {
	AddonsRegisterMap = map[string]addonsLoader{}
	// register inTree addons
	//AddonsRegisterMap[Prometheus.String()] = &prometheusAddons{}
	//AddonsRegisterMap[Elasticsearch.String()] = &esAddons{}
	//AddonsRegisterMap[Ingress.String()] = &ingressAddons{}
	AddonsRegisterMap[ApiServer.String()] = &apiServerAddons{}
	AddonsRegisterMap[Etcd.String()] = &etcdAddons{}
}

// load inTree plugins data
func LoadInTreeData(name string) (*model.PluginsData, error) {
	loader, ok := AddonsRegisterMap[strings.ToLower(name)]
	if !ok || loader == nil {
		return nil, errors.New(fmt.Sprintf("can not find inTree(%s)", name))
	}
	return loader.Load()
}
