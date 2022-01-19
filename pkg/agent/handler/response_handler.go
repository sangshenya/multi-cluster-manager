package handler

import (
	"errors"

	agentStream "harmonycloud.cn/stellaris/pkg/agent/stream"
	"harmonycloud.cn/stellaris/pkg/model"
)

/*
const (
	Unknown                     ServiceResponseType = "Unknown"
	Error                       ServiceResponseType = "Error"
	RegisterSuccess             ServiceResponseType = "RegisterSuccess"
	RegisterFailed              ServiceResponseType = "RegisterFailed"
	HeartbeatSuccess            ServiceResponseType = "HeartbeatSuccess"
	HeartbeatFailed             ServiceResponseType = "HeartbeatFailed"
	ResourceUpdateOrCreate      ServiceResponseType = "ResourceUpdateOrCreate"
	ResourceDelete              ServiceResponseType = "ResourceDelete"
	ResourceStatusUpdateSuccess ServiceResponseType = "ResourceStatusUpdateSuccess"
	ResourceStatusUpdateFailed  ServiceResponseType = "ResourceStatusUpdateFailed"
)
*/

func RecvResponse() {
	for {
		stream := agentStream.GetConnection("")
		if stream == nil {
			err := errors.New("get stream failed")
			registerLog.Error(err, "recv response")
			continue
		}
		response, err := stream.Recv()
		if err != nil {
			registerLog.Error(err, "recv response failed")
			continue
		}
		switch response.Type {
		case model.Unknown.String():
		case model.Error.String():
		case model.RegisterSuccess.String():
		case model.RegisterFailed.String():
		case model.HeartbeatSuccess.String():
		case model.HeartbeatFailed.String():
		case model.ResourceUpdateOrCreate.String():
		case model.ResourceDelete.String():
		case model.ResourceStatusUpdateSuccess.String():
		case model.ResourceStatusUpdateFailed.String():

		}
	}
}
