package token

import (
	"context"
	"errors"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"harmonycloud.cn/stellaris/pkg/common/helper"
	v1 "k8s.io/api/core/v1"
)

func ValidateToken(ctx context.Context, clientSet client.Client, token string) error {
	if len(token) == 0 {
		return errors.New("token validate failed, token is empty")
	}

	coreToken := getToken(ctx, clientSet)
	if token != coreToken {
		return errors.New("validate token failed")
	}
	return nil
}

func getToken(ctx context.Context, clientSet client.Client) string {
	name, err := helper.GetOperatorName()
	if err != nil {
		name = "stellaris-core"
	}
	name = name + "-register-token"
	namespace, err := helper.GetOperatorNamespace()
	if err != nil {
		namespace = "stellaris-system"
	}

	tokenCm := &v1.ConfigMap{}

	err = clientSet.Get(ctx, types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}, tokenCm)
	if err != nil {
		return ""
	}

	token, ok := tokenCm.Data["token"]
	if !ok {
		return ""
	}
	return token
}
