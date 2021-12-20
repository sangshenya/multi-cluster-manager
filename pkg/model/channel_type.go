package model

type AddonsChannel struct {
	Channels        []chan *Addon
	MonitorChannels []chan *Condition
}
