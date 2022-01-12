package stream

import (
	"fmt"
	"sync"
	"time"

	"harmonycloud.cn/stellaris/config"
)

var table map[string]*Stream

var lock sync.RWMutex

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
	lock.Lock()
	defer lock.Unlock()
	// TODO insert table should has no health stream
	if table[clusterName] != nil && table[clusterName].Status == OK {
		return fmt.Errorf("failed insert stream table")
	}
	table[clusterName] = stream
	return nil
}

func FindStream(clusterName string) *Stream {
	lock.RLock()
	defer lock.RUnlock()
	return table[clusterName]
}
