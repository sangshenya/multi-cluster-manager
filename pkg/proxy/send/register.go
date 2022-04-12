package send

import (
	"context"
	"errors"
	"fmt"

	"harmonycloud.cn/stellaris/pkg/model"
	"harmonycloud.cn/stellaris/pkg/proxy/addons"
	proxy_cfg "harmonycloud.cn/stellaris/pkg/proxy/config"
	proxy_stream "harmonycloud.cn/stellaris/pkg/proxy/stream"
	"harmonycloud.cn/stellaris/pkg/utils/common"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var registerLog = logf.Log.WithName("proxy_send_register")

func Register() error {
	registerLog.Info(fmt.Sprintf("start register cluster(%s)", proxy_cfg.ProxyConfig.Cfg.ClusterName))
	stream := proxy_stream.GetConnection()
	if stream == nil {
		err := errors.New("get stream failed")
		registerLog.Error(err, "register")
		return err
	}
	addonInfo := &model.RegisterRequest{
		Token: proxy_cfg.ProxyConfig.Cfg.RegisterToken,
	}

	addonsList := addons.LoadAddon(context.Background(), proxy_cfg.ProxyConfig.ControllerClient)
	if len(addonsList) > 0 {
		addonInfo.Addons = addonsList
	}

	request, err := common.GenerateRequest(model.Register.String(), addonInfo, proxy_cfg.ProxyConfig.Cfg.ClusterName)
	if err != nil {
		registerLog.Error(err, "create request failed")
		return err
	}
	if err := stream.Send(request); err != nil {
		registerLog.Error(err, "send request failed")
		return err
	}

	return nil
}
