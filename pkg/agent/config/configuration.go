package config

import "time"

type Configuration struct {
	HeartbeatPeriod time.Duration
	ClusterName     string
	CoreAddress     string
	AddonPath       string
}

func DefaultConfiguration() *Configuration {
	return &Configuration{}
}
