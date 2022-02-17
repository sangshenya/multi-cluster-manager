package controller

import (
	clusterController "harmonycloud.cn/stellaris/pkg/controller/cluster"
	clusterResourceController "harmonycloud.cn/stellaris/pkg/controller/cluster-resource"
	clusterSetController "harmonycloud.cn/stellaris/pkg/controller/cluster-set"
	controllerCommon "harmonycloud.cn/stellaris/pkg/controller/common"
	multiClusterRsourceController "harmonycloud.cn/stellaris/pkg/controller/multi-cluster-resource"
	namespaceMappingController "harmonycloud.cn/stellaris/pkg/controller/namespace-mapping"
	resourceBindingController "harmonycloud.cn/stellaris/pkg/controller/resource-binding"
	resourceSchedulePolicyController "harmonycloud.cn/stellaris/pkg/controller/resource-schedule-policy"
	ctrl "sigs.k8s.io/controller-runtime"
)

func Setup(mgr ctrl.Manager, args controllerCommon.Args) error {
	controllerSetupFunctions := []func(ctrl.Manager, controllerCommon.Args) error{
		clusterSetController.Setup,
		namespaceMappingController.Setup,
		clusterController.Setup,
		resourceBindingController.Setup,
		multiClusterRsourceController.Setup,
		clusterResourceController.Setup,
		resourceSchedulePolicyController.Setup,
	}
	if !args.IsControlPlane {
		controllerSetupFunctions = []func(ctrl.Manager, controllerCommon.Args) error{
			clusterResourceController.Setup,
		}
	}

	for _, setupFunc := range controllerSetupFunctions {
		if err := setupFunc(mgr, args); err != nil {
			return err
		}
	}
	return nil
}
