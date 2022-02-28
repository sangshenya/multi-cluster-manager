package handler

import "harmonycloud.cn/stellaris/pkg/model"

func changeResponseTypeToAggregateType(responseType string) model.SyncAggregateResourceType {
	switch responseType {
	case model.AggregateDelete.String():
		return model.DeleteResource
	case model.AggregateUpdateOrCreate.String():
		return model.UpdateOrCreateResource
	case model.RegisterSuccess.String():
		return model.SyncResource
	}
	return model.UnknownType
}
