package agent

import (
	"context"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"harmonycloud.cn/stellaris/config"
	agentconfig "harmonycloud.cn/stellaris/pkg/agent/config"
	"harmonycloud.cn/stellaris/pkg/model"
)

func GetAddonConfig(path string) (*model.PluginsConfig, error) {
	var configViperConfig = viper.New()
	configViperConfig.SetConfigFile(path)

	if err := configViperConfig.ReadInConfig(); err != nil {
		return nil, err
	}
	var c model.PluginsConfig
	if err := configViperConfig.Unmarshal(&c); err != nil {
		return nil, err
	}
	return &c, nil
}

func Connection(cfg *agentconfig.Configuration) (config.Channel_EstablishClient, error) {
	conn, err := grpc.Dial(cfg.CoreAddress, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	grpcClient := config.NewChannelClient(conn)
	stream, err := grpcClient.Establish(context.Background())
	if err != nil {
		return nil, err
	}
	return stream, nil
}
