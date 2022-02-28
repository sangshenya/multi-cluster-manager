package proxy

import (
	"github.com/spf13/viper"
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
