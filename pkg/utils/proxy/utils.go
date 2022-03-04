package proxy

import (
	"github.com/spf13/viper"
	"harmonycloud.cn/stellaris/pkg/model"
)

func GetAddonConfig(path string) (*model.AddonsConfig, error) {
	var configViperConfig = viper.New()
	configViperConfig.SetConfigFile(path)

	if err := configViperConfig.ReadInConfig(); err != nil {
		return nil, err
	}
	var cfg model.AddonsConfig
	if err := configViperConfig.Unmarshal(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
