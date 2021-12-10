package stream

import (
	"fmt"
	"sync"
	"time"

	"harmonycloud.cn/stellaris/config"
)

var table map[string]*Stream

const (
	OK     = "ok"
	Expire = "expire"
)

type Stream struct {
	ClusterName string
	Stream      config.Channel_EstablishServer
	Status      string
	Expire      time.Time
}

func init() {
	table = make(map[string]*Stream)
}

func Insert(clusterName string, stream *Stream) error {
	mu := sync.Mutex{}
	mu.Lock()
	// TODO insert table should has no health stream
	if table[clusterName] != nil && table[clusterName].Status == OK {
		return fmt.Errorf("failed insert stream table")
	}
	table[clusterName] = stream
	mu.Unlock()
	return nil
}

func FindStream(clusterName string) *Stream {
	return table[clusterName]
}
