package addons

import (
	"sync"

	"harmonycloud.cn/stellaris/pkg/model"
)

type AddonManager struct {
	addonMap map[string]model.Addon
	sync.RWMutex
}

func NewAddonManager() *AddonManager {
	m := &AddonManager{}
	m.addonMap = make(map[string]model.Addon)
	return m
}

func (a *AddonManager) AppendAddon(addon model.Addon) {
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
		addonList = append(addonList, v)
	}
	return addonList
}
