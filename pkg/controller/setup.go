package controller

import (
	clusterController "harmonycloud.cn/stellaris/pkg/controller/cluster"
	controllerCommon "harmonycloud.cn/stellaris/pkg/controller/common"
	ctrl "sigs.k8s.io/controller-runtime"
)

func Setup(mgr ctrl.Manager, args controllerCommon.Args) error {
	controllerSetupFunctions := []func(ctrl.Manager, controllerCommon.Args) error{
		clusterController.Setup,
	}

	for _, setupFunc := range controllerSetupFunctions {
		if err := setupFunc(mgr, args); err != nil {
			return err
		}
	}
	return nil
}
