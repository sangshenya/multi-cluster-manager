package config

import "time"

type Configuration struct {
	HeartbeatExpirePeriod    time.Duration
	OnlineExpirationTime     time.Duration
	ClusterStatusCheckPeriod time.Duration
}

func DefaultConfiguration() *Configuration {
	return &Configuration{}
}
