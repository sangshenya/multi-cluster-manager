package common

import (
	"encoding/json"
	"fmt"
	"harmonycloud.cn/stellaris/config"
)

func GenerateRequest(sendType string, v interface{}, clusterName string) (*config.Request, error) {
	requestBody, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("marshal err: %v", err)
	}
	request := &config.Request{
		Type:        sendType,
		ClusterName: clusterName,
		Body:        string(requestBody),
	}
	return request, nil

}
