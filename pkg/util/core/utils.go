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

func SendErrResponse(clusterName string, err error, stream config.Channel_EstablishServer) {
	res := &config.Response{
		Type:        "Error",
		ClusterName: clusterName,
		Body:        err.Error(),
	}
	SendResponse(res, stream)
}

func convertRegisterRequest2Cluster(req *config.Request) (*v1alpha1.Cluster, error) {
	data := &model.RegisterRequest{}
	if err := json.Unmarshal([]byte(req.Body), data); err != nil {
		return nil, err
	}

	return &v1alpha1.Cluster{
		ObjectMeta: v1.ObjectMeta{
			Name: req.ClusterName,
		},
		Spec: v1alpha1.ClusterSpec{
			Addons: nil,
		},
	}, nil
}

func ConvertRegisterAddons2KubeAddons(addons []model.Addon) ([]v1alpha1.ClusterAddons, error) {
	result := make([]v1alpha1.ClusterAddons, len(addons))
	for _, addon := range addons {
		raw, err := Object2RawExtension(addon.Properties)
		if err != nil {
			return nil, err
		}
		clusterAddon := v1alpha1.ClusterAddons{
			Name: addon.Name,
			Info: raw,
		}
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
