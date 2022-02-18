package cur_render

import (
	"context"
	"fmt"
	"math/rand"
	"strconv"
	"sync"
	"testing"
	"time"
)

type CancelContext struct {
	Ctx    context.Context
	Cancel context.CancelFunc
}

func TestChannels(t *testing.T) {

	ctxList := []CancelContext{}
	for i := 0; i < 5; i++ {
		cancelCtx, cancel := context.WithCancel(context.Background())
		ctxList = append(ctxList, CancelContext{
			Ctx:    cancelCtx,
			Cancel: cancel,
		})
	}

	wg := sync.WaitGroup{}
	for count, item := range ctxList {
		wg.Add(1)
		// 模拟用户取消
		go func(i int, ctxObject CancelContext) {
			time.Sleep(time.Duration(i) * time.Second)
			ctxObject.Cancel()
		}(count+1, item)
		// 模拟执行任务
		go func(i int, ctxObject CancelContext) {
			str, err := test(ctxObject.Ctx, i)
			if err != nil {
				fmt.Println("get error:", err)
			} else {
				fmt.Println("get result:", str)
			}
			wg.Done()
		}(count+1, item)
	}
	wg.Wait()

}

func test(ctx context.Context, count int) (string, error) {
	var result string
	var err error

	// 模拟任务耗时
	go func() {
		index := rand.Int()%5 + 2
		s := "正常执行会被返回" + strconv.Itoa(index)
		if count < index {
			s = "耗时太久，将会被取消"
		}
		fmt.Println("spendTime:", index, "waitTime:", count, s)
		time.Sleep(time.Duration(index) * time.Second)
		result = s
	}()

	timeout := make(chan struct{}, 1)
	go func() {
		for {
			// 被取消
			if ctx.Err() == context.Canceled {
				err = ctx.Err()
				timeout <- struct{}{}
			}
			// 获取到值
			if len(result) != 0 || err != nil {
				timeout <- struct{}{}
			}
		}
	}()

	select {
	case <-timeout:
		return result, err
	}
}
