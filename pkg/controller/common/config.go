package common

import (
	clientset "harmonycloud.cn/stellaris/pkg/client/clientset/versioned"
	"k8s.io/apimachinery/pkg/types"
)

type Args struct {
	ManagerClientSet   *clientset.Clientset
	IsControlPlane     bool
	TmplNamespacedName types.NamespacedName
}
