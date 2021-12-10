package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"harmonycloud.cn/stellaris/config"
	agentconfig "harmonycloud.cn/stellaris/pkg/agent/config"
	addoninfo "harmonycloud.cn/stellaris/pkg/model"
)

func Register(cfg *agentconfig.Configuration) error {
	conn, err := grpc.Dial(cfg.CoreAddress, grpc.WithInsecure())
	if err != nil {
		return fmt.Errorf("grpc conn err: %v", err)
	}
	grpcClient := config.NewChannelClient(conn)
	stream, err := grpcClient.Establish(context.Background())
	if err != nil {
		return fmt.Errorf("call err: %v", err)
	}
	addonInfo, err := GetAddonConfig(cfg.AddonPath)
	if err != nil {
		return fmt.Errorf("get addons config err: %v", err)
	}
	addonRequest, err := json.Marshal(addonInfo)
	if err != nil {
		return fmt.Errorf("marshal err: %v", err)
	}

	request := &config.Request{
		Type:        "Register",
		ClusterName: cfg.ClusterName,
		Body:        string(addonRequest),
	}

	if err = stream.Send(request); err != nil {
		return fmt.Errorf("stream send to server err: %v", err)
	}

	resp, err := stream.Recv()
	if err != nil {
		return fmt.Errorf("stream get from server err: %v", err)
	}
	logrus.Printf("stream get from server:%v", resp)
	//TODO After Receive Response
	return nil

}

func GetAddonConfig(path string) (*addoninfo.RegisterRequest, error) {
	var configViperConfig = viper.New()
	configViperConfig.SetConfigFile(path)

	if err := configViperConfig.ReadInConfig(); err != nil {
		return nil, err
	}
	var c addoninfo.RegisterRequest
	if err := configViperConfig.Unmarshal(&c); err != nil {
		return nil, err
	}
	return &c, nil

}
