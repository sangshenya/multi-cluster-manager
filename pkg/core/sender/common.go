package sender

import (
	"errors"

	"harmonycloud.cn/stellaris/config"
	"harmonycloud.cn/stellaris/pkg/model"
)

func NewResponse(resType model.ServiceResponseType, clusterName string, body string) (*config.Response, error) {
	if len(clusterName) == 0 || len(body) == 0 {
		return nil, errors.New("clusterName or body is empty")
	}
	return &config.Response{
		Type:        resType.String(),
		ClusterName: clusterName,
		Body:        body,
	}, nil
}
