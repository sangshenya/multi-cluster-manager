package addons

import (
	"context"
	"time"

	agentconfig "harmonycloud.cn/stellaris/pkg/agent/config"

	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"harmonycloud.cn/stellaris/pkg/model"
)

var addonsLog = logf.Log.WithName("agent_addon")

func LoadAddon(cfg *model.PluginsConfig) []model.Addon {
	if len(cfg.Plugins.InTree) <= 0 && len(cfg.Plugins.OutTree) <= 0 {
		return []model.Addon{}
	}

	deadline := time.Now().Add(agentconfig.AgentConfig.Cfg.AddonLoadTimeout)
	deadlineCtx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()

	addCh := model.AddonsChannel{}
	for _, inTree := range cfg.Plugins.InTree {
		channels := make(chan *model.Addon)
		addCh.Channels = append(addCh.Channels, channels)

		go runPlugins(inTree.Name, "", channels)
	}

	for _, outTree := range cfg.Plugins.OutTree {
		channels := make(chan *model.Addon)
		addCh.Channels = append(addCh.Channels, channels)

		go runPlugins(outTree.Name, outTree.Url, channels)
	}

	addonsInfo := getAddonsInfo(deadlineCtx, addCh)
	return addonsInfo
}

func getAddon(name string, url string) *model.Addon {
	if len(name) == 0 {
		return nil
	}
	res := &model.Addon{}
	if len(url) != 0 {
		// TODO get OutTreePlugins
	}
	// TODO get InTreePlugins
	return res
}

func runPlugins(name, url string, ch chan *model.Addon) {
	res := getAddon(name, url)
	ch <- res
}

func getAddonsInfo(ctx context.Context, addCh model.AddonsChannel) []model.Addon {
	am := NewAddonManager()

	for _, ch := range addCh.Channels {
		go func(addonCh chan *model.Addon) {
			addon := <-addonCh
			am.AppendAddon(*addon)
		}(ch)
	}

	stop := make(chan struct{}, 1)
	go func() {
		for {
			if ctx.Err() == context.DeadlineExceeded {
				stop <- struct{}{}
			}
			if am.Len() == len(addCh.Channels) {
				stop <- struct{}{}
			}
		}
	}()

	select {
	case <-stop:
		return am.AddonList()
	}

}
