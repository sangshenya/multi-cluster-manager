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
		stream := agentStream.GetConnection()
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
			RecvRegisterResponse(response)
		case model.RegisterFailed.String():
			RecvRegisterResponse(response)
		case model.HeartbeatSuccess.String():
			RecvHeartbeatResponse(response)
		case model.HeartbeatFailed.String():
			RecvHeartbeatResponse(response)
		case model.ResourceUpdateOrCreate.String():
			RecvSyncResourceResponse(response)
		case model.ResourceDelete.String():
			RecvSyncResourceResponse(response)
		case model.ResourceStatusUpdateSuccess.String():
			RecvSyncResourceResponse(response)
		case model.ResourceStatusUpdateFailed.String():
			RecvSyncResourceResponse(response)

		}
	}
}
