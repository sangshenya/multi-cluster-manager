package model

type ServiceRequestType string

const (
	Register  ServiceRequestType = "Register"
	Heartbeat ServiceRequestType = "Heartbeat"
	Resource  ServiceRequestType = "Resource"
	Aggregate ServiceRequestType = "Aggregate"
)

func (s ServiceRequestType) String() string {
	return string(s)
}

type ServiceResponseType string

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

func (s ServiceResponseType) String() string {
	return string(s)
}
