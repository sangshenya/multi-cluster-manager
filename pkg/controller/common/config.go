package common

import clientset "harmonycloud.cn/stellaris/pkg/client/clientset/versioned"

type Args struct {
	ManagerClientSet *clientset.Clientset
}
