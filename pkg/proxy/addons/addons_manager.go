package addons

import (
	"strings"
	"sync"

	"k8s.io/apimachinery/pkg/util/rand"

	"harmonycloud.cn/stellaris/pkg/model"
)

type AddonManager struct {
	addonMap map[string]*model.AddonsData
	sync.RWMutex
}

var emptyAddonPrefix = "empty-"

func NewAddonManager() *AddonManager {
	m := &AddonManager{}
	m.addonMap = make(map[string]*model.AddonsData)
	return m
}

func (a *AddonManager) AppendAddon(addon *model.AddonsData) {
	if addon == nil {
		addon = randEmptyAddon()
	}
	a.RLock()
	_, ok := a.addonMap[addon.Name]
	a.RUnlock()
	if ok {
		return
	}
	a.Lock()
	defer a.Unlock()
	a.addonMap[addon.Name] = addon
}

func (a *AddonManager) Len() int {
	a.RLock()
	defer a.RUnlock()
	return len(a.addonMap)
}

func (a *AddonManager) AddonList() []model.AddonsData {
	var addonList []model.AddonsData
	a.RLock()
	defer a.RUnlock()
	for _, v := range a.addonMap {
		if v != nil && !isEmptyAddon(v) {
			addonList = append(addonList, *v)
		}
	}
	return addonList
}

func randEmptyAddon() *model.AddonsData {
	return &model.AddonsData{
		Name: emptyAddonPrefix + rand.String(8),
		Info: nil,
	}
}

func isEmptyAddon(addon *model.AddonsData) bool {
	return strings.HasPrefix(addon.Name, emptyAddonPrefix)
}
