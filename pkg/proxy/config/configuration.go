package config

import "time"

type Configuration struct {
	HeartbeatPeriod  time.Duration
	ClusterName      string
	CoreAddress      string
	AddonPath        string
	RegisterToken    string
	AddonLoadTimeout time.Duration
}

func DefaultConfiguration() *Configuration {
	return &Configuration{}
}
