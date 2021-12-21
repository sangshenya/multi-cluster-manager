package controller

import (
	clusterController "harmonycloud.cn/stellaris/pkg/controller/cluster"
	controllerCommon "harmonycloud.cn/stellaris/pkg/controller/common"
	multiClusterRsourceController "harmonycloud.cn/stellaris/pkg/controller/multi_cluster_resource"
	resourceBindingController "harmonycloud.cn/stellaris/pkg/controller/resource_binding"
	ctrl "sigs.k8s.io/controller-runtime"
)

func Setup(mgr ctrl.Manager, args controllerCommon.Args) error {
	controllerSetupFunctions := []func(ctrl.Manager, controllerCommon.Args) error{
		clusterController.Setup,
		resourceBindingController.Setup,
		multiClusterRsourceController.Setup,
	}

	for _, setupFunc := range controllerSetupFunctions {
		if err := setupFunc(mgr, args); err != nil {
			return err
		}
	}
	return nil
}
