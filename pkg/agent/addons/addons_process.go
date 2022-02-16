package addons

import (
	"context"
	"fmt"
	"time"

	"harmonycloud.cn/stellaris/pkg/agent/addons/inTree"

	"harmonycloud.cn/stellaris/pkg/agent/addons/outTree"

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
	for _, in := range cfg.Plugins.InTree {
		channels := make(chan *model.Addon)
		addCh.Channels = append(addCh.Channels, channels)

		go runPlugins(in.Name, "", channels)
	}

	for _, out := range cfg.Plugins.OutTree {
		channels := make(chan *model.Addon)
		addCh.Channels = append(addCh.Channels, channels)

		go runPlugins(out.Name, out.Url, channels)
	}

	addonsInfo := getAddonsInfo(deadlineCtx, addCh)
	return addonsInfo
}

func getAddon(name string, url string) *model.Addon {
	if len(name) == 0 {
		return nil
	}
	res := &model.Addon{
		Name: name,
	}
	if len(url) != 0 {
		// load outTree data
		outTreeData, err := outTree.LoadOutTreeData(url)
		if err != nil || outTreeData == nil {
			addonsLog.Error(err, fmt.Sprintf("get outTree plugin(%s) info failed", url))
			return nil
		}
		res.Properties = outTreeData
		return res
	}
	// load inTree data
	inTreeData, err := inTree.LoadInTreeData(name)
	if err != nil || inTreeData == nil {
		addonsLog.Error(err, fmt.Sprintf("get inTree plugin(%s) info failed", name))
		return nil
	}
	res.Properties = inTreeData
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
			am.AppendAddon(addon)
		}(ch)
	}

	stop := make(chan struct{}, 1)
	go func() {
		for {
			// cancel for timeout
			if ctx.Err() == context.DeadlineExceeded {
				stop <- struct{}{}
			}
			// cancel for load finish
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
