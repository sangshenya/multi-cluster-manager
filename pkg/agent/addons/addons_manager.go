package addons

import (
	"strings"
	"sync"

	"k8s.io/apimachinery/pkg/util/rand"

	"harmonycloud.cn/stellaris/pkg/model"
)

type AddonManager struct {
	addonMap map[string]*model.Addon
	sync.RWMutex
}

var emptyAddonPrefix = "empty-"

func NewAddonManager() *AddonManager {
	m := &AddonManager{}
	m.addonMap = make(map[string]*model.Addon)
	return m
}

func (a *AddonManager) AppendAddon(addon *model.Addon) {
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

func (a *AddonManager) AddonList() []model.Addon {
	var addonList []model.Addon
	a.RLock()
	defer a.RUnlock()
	for _, v := range a.addonMap {
		if v != nil && !isEmptyAddon(v) {
			addonList = append(addonList, *v)
		}
	}
	return addonList
}

func randEmptyAddon() *model.Addon {
	return &model.Addon{
		Name:       emptyAddonPrefix + rand.String(8),
		Properties: nil,
	}
}

func isEmptyAddon(addon *model.Addon) bool {
	return strings.HasPrefix(addon.Name, emptyAddonPrefix)
}
