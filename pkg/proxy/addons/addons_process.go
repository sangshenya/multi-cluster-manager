package addons

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"harmonycloud.cn/stellaris/pkg/apis/multicluster/v1alpha1"
	"harmonycloud.cn/stellaris/pkg/common/helper"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	inTree "harmonycloud.cn/stellaris/pkg/proxy/addons/in-tree"

	outTree "harmonycloud.cn/stellaris/pkg/proxy/addons/out-tree"

	proxy_cfg "harmonycloud.cn/stellaris/pkg/proxy/config"

	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"harmonycloud.cn/stellaris/pkg/model"
)

var addonsLog = logf.Log.WithName("proxy_addon")

func LoadAddon(ctx context.Context, clientSet client.Client) []model.AddonsData {
	cfg, err := getAddonsConfig(ctx, clientSet)
	if err != nil {
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
		addonsLog.Error(err, fmt.Sprintf("get outTree addons(%s) info failed", out.Name))
		return nil
	}
	return outTreeData
}

func getInTreeAddon(ctx context.Context, in *model.In) *model.AddonsData {
	inTreeData, err := inTree.LoadInTreeData(ctx, in)
	if err != nil || inTreeData == nil {
		addonsLog.Error(err, fmt.Sprintf("get inTree addons(%s) info failed", in.Name))
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

func getAddonsConfig(ctx context.Context, clientSet client.Client) (*model.AddonsConfig, error) {
	// get cm
	var cmName, cmNamespace string
	var err error
	cmName, err = helper.GetOperatorName()
	if err != nil {
		return nil, err
	}
	cmNamespace, err = helper.GetOperatorNamespace()
	if err != nil {
		return nil, err
	}

	namespaced := types.NamespacedName{
		Namespace: cmNamespace,
		Name:      cmName,
	}
	cm := &corev1.ConfigMap{}
	err = clientSet.Get(ctx, namespaced, cm)
	if err != nil {
		return nil, err
	}

	addonsCmConfig := &model.AddonsCmConfig{}
	for _, value := range cm.Data {
		err = json.Unmarshal([]byte(value), addonsCmConfig)
		if err != nil {
			return nil, err
		}
		if len(addonsCmConfig.Addons) > 0 {
			break
		}
	}
	addons := &model.Addons{}
	for _, addon := range addonsCmConfig.Addons {
		if addon.Type == v1alpha1.InTreeType {
			inTreeCfg, err := unmarshalInTreeAddons(addon.Configuration)
			if err != nil {
				continue
			}
			addons.InTree = append(addons.InTree, model.In{
				Name:           addon.Name,
				Configurations: inTreeCfg,
			})
			continue
		}
		outTreeCfg, err := unmarshalOutTreeAddons(addon.Configuration)
		if err != nil {
			continue
		}
		addons.OutTree = append(addons.OutTree, model.Out{
			Name:           addon.Name,
			Configurations: outTreeCfg,
		})
	}
	if len(addons.OutTree) == 0 && len(addons.InTree) == 0 {
		return nil, errors.New("addon config is empty")
	}
	return &model.AddonsConfig{
		Addons: addons,
	}, nil
}

func unmarshalInTreeAddons(configuration *runtime.RawExtension) (*model.InTreeConfig, error) {
	addonCfgData, err := configuration.MarshalJSON()
	if err != nil {
		return nil, err
	}
	inTreeCfg := &model.InTreeConfig{}
	err = json.Unmarshal(addonCfgData, inTreeCfg)
	if err != nil {
		return nil, err
	}
	return inTreeCfg, nil
}

func unmarshalOutTreeAddons(configuration *runtime.RawExtension) (*model.OutTreeConfig, error) {
	addonCfgData, err := configuration.MarshalJSON()
	if err != nil {
		return nil, err
	}
	outTreeCfg := &model.OutTreeConfig{}
	err = json.Unmarshal(addonCfgData, outTreeCfg)
	if err != nil {
		return nil, err
	}
	return outTreeCfg, nil
}
