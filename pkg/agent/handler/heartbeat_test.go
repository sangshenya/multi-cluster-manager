package handler

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	"harmonycloud.cn/stellaris/pkg/model"
)

func TestChannels(t *testing.T) {
	deadline := time.Now().Add(5 * time.Second)
	deadlineCtx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()
	list := []int{1, 2, 3, 4, 5, 6}

	addCh := model.AddonsChannel{}
	for _, item := range list {
		channels := make(chan *model.Addon)
		addCh.Channels = append(addCh.Channels, channels)

		go func(i int, ch chan *model.Addon) {
			time.Sleep(time.Duration(i) * time.Second)
			addon := &model.Addon{
				Name: strconv.Itoa(i),
			}
			fmt.Println(time.Now())
			channels <- addon
		}(item, channels)
	}

	addonList := test(deadlineCtx, addCh)
	fmt.Println(len(addonList))
	for _, item := range addonList {
		fmt.Println(item.Name)
	}

}

func test(ctx context.Context, addCh model.AddonsChannel) []*model.Addon {
	var addonList []*model.Addon
	for _, ch := range addCh.Channels {
		go func(ch1 chan *model.Addon) {
			addon := <-ch1
			addonList = append(addonList, addon)
		}(ch)
	}

	timeout := make(chan struct{}, 1)
	go func() {
		for {
			if ctx.Err() == context.DeadlineExceeded {
				fmt.Println("111:", time.Now())
				timeout <- struct{}{}
			}
		}
	}()

	select {
	case <-timeout:
		return addonList
	}
}
