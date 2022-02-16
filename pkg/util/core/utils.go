package core

import (
	"encoding/json"

	"harmonycloud.cn/stellaris/pkg/apis/multicluster/common"

	"github.com/sirupsen/logrus"
	"harmonycloud.cn/stellaris/config"
	"harmonycloud.cn/stellaris/pkg/apis/multicluster/v1alpha1"
	"harmonycloud.cn/stellaris/pkg/model"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func SendResponse(res *config.Response, stream config.Channel_EstablishServer) {
	if err := stream.Send(res); err != nil {
		logrus.Errorf("failed to send message to cluster %s", err)
	}
}

func SendErrResponse(clusterName string, responseType model.ServiceResponseType, err error, stream config.Channel_EstablishServer) {
	res := &config.Response{
		Type:        responseType.String(),
		ClusterName: clusterName,
		Body:        err.Error(),
	}
	SendResponse(res, stream)
}

func NewCluster(clusterName string) *v1alpha1.Cluster {
	return &v1alpha1.Cluster{
		ObjectMeta: v1.ObjectMeta{
			Name: clusterName,
		},
		Spec: v1alpha1.ClusterSpec{
			Addons: nil,
		},
	}
}

func ConvertRegisterAddons2KubeAddons(addons []model.Addon) ([]v1alpha1.ClusterAddons, error) {
	result := make([]v1alpha1.ClusterAddons, len(addons))
	for _, addon := range addons {
		if len(addon.Name) <= 0 {
			continue
		}
		clusterAddon := v1alpha1.ClusterAddons{
			Name: addon.Name,
		}
		if addon.Properties == nil {
			result = append(result, clusterAddon)
			continue
		}
		raw, err := Object2RawExtension(addon.Properties)
		if err != nil {
			return nil, err
		}
		clusterAddon.Info = raw
		result = append(result, clusterAddon)
	}
	return result, nil
}

func ConvertCondition2KubeCondition(conditions []model.Condition) []common.Condition {
	result := make([]common.Condition, len(conditions))
	for _, condition := range conditions {
		clusterCondition := common.Condition{
			Timestamp: condition.Timestamp,
			Message:   condition.Message,
			Reason:    condition.Reason,
			Type:      condition.Type,
		}
		result = append(result, clusterCondition)
	}
	return result
}

func Object2RawExtension(obj interface{}) (*runtime.RawExtension, error) {
	b, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}
	return &runtime.RawExtension{
		Raw: b,
	}, nil
}
