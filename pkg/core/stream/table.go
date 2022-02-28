package stream

import (
	"fmt"
	"sync"
	"time"

	logf "sigs.k8s.io/controller-runtime/pkg/log"

	timeutil "harmonycloud.cn/stellaris/pkg/utils/time"

	"harmonycloud.cn/stellaris/config"
)

var table map[string]*Stream

var lock sync.RWMutex

var tableLog = logf.Log.WithName("core_table")

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

func Insert(clusterName string, stream *Stream) {
	lock.RLock()
	existStream, _ := table[clusterName]
	lock.RUnlock()

	lock.Lock()
	defer lock.Unlock()
	if existStream != nil && existStream.Status == OK && !existStream.isExpire() {
		return
	}
	table[clusterName] = stream
	tableLog.Info(fmt.Sprintf("insert proxy(%s) stream success", clusterName))
}

func FindStream(clusterName string) *Stream {
	lock.RLock()
	defer lock.RUnlock()
	return table[clusterName]
}

func (s *Stream) isExpire() bool {
	return timeutil.NowTimeWithLoc().Sub(s.Expire) > 0
}
