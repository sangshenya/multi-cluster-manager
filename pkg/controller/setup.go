package controller

import (
	clusterController "harmonycloud.cn/stellaris/pkg/controller/cluster"
	clusterResourceController "harmonycloud.cn/stellaris/pkg/controller/cluster_resource"
	clusterSetController "harmonycloud.cn/stellaris/pkg/controller/cluster_set"
	controllerCommon "harmonycloud.cn/stellaris/pkg/controller/common"
	multiClusterRsourceController "harmonycloud.cn/stellaris/pkg/controller/multi_cluster_resource"
	namespaceMappingController "harmonycloud.cn/stellaris/pkg/controller/namespace_mapping"
	resourceBindingController "harmonycloud.cn/stellaris/pkg/controller/resource_binding"
	resourceSchedulePolicyController "harmonycloud.cn/stellaris/pkg/controller/resource_schedule_policy"
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

	for _, setupFunc := range controllerSetupFunctions {
		if err := setupFunc(mgr, args); err != nil {
			return err
		}
	}
	return nil
}
