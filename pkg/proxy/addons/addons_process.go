package addons

import (
	"context"
	"fmt"
	"time"

	inTree "harmonycloud.cn/stellaris/pkg/proxy/addons/in-tree"

	outTree "harmonycloud.cn/stellaris/pkg/proxy/addons/out-tree"

	proxy_cfg "harmonycloud.cn/stellaris/pkg/proxy/config"

	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"harmonycloud.cn/stellaris/pkg/model"
)

var addonsLog = logf.Log.WithName("proxy_addon")

func LoadAddon(cfg *model.AddonsConfig) []model.AddonsData {
	if cfg.Addons == nil || (len(cfg.Addons.InTree) == 0 && len(cfg.Addons.OutTree) == 0) {
		return []model.AddonsData{}
	}

	deadline := time.Now().Add(proxy_cfg.ProxyConfig.Cfg.AddonLoadTimeout)
	deadlineCtx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()

	addCh := model.AddonsChannel{}
	for _, in := range cfg.Addons.InTree {

		channels := make(chan *model.AddonsData)
		addCh.Channels = append(addCh.Channels, channels)

		go func(ch chan *model.AddonsData, inTreeCfg model.In) {
			res := getInTreeAddon(deadlineCtx, &inTreeCfg)
			ch <- res
		}(channels, in)
	}

	for _, out := range cfg.Addons.OutTree {
		channels := make(chan *model.AddonsData)
		addCh.Channels = append(addCh.Channels, channels)

		go func(ch chan *model.AddonsData, outTreeCfg model.Out) {
			res := getOutTreeAddon(deadlineCtx, &outTreeCfg)
			ch <- res
		}(channels, out)
	}

	addonsInfo := getAddonsInfo(deadlineCtx, addCh)
	return addonsInfo
}

func getOutTreeAddon(ctx context.Context, out *model.Out) *model.AddonsData {
	// load outTree data
	outTreeData, err := outTree.LoadOutTreeData(ctx, out)
	if err != nil || outTreeData == nil {
		addonsLog.Error(err, fmt.Sprintf("get outTree plugin(%s) info failed", out.Name))
		return nil
	}
	return outTreeData
}

func getInTreeAddon(ctx context.Context, in *model.In) *model.AddonsData {
	inTreeData, err := inTree.LoadInTreeData(ctx, in)
	if err != nil || inTreeData == nil {
		addonsLog.Error(err, fmt.Sprintf("get inTree plugin(%s) info failed", in.Name))
		return nil
	}
	return inTreeData
}

func getAddonsInfo(ctx context.Context, addCh model.AddonsChannel) []model.AddonsData {
	am := NewAddonManager()
	for _, ch := range addCh.Channels {
		go func(addonCh chan *model.AddonsData) {
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

	<-stop
	return am.AddonList()
}
