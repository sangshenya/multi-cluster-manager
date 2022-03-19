package controller

import (
	clusterController "harmonycloud.cn/stellaris/pkg/controller/cluster"
	clusterResourceController "harmonycloud.cn/stellaris/pkg/controller/cluster-resource"
	clusterSetController "harmonycloud.cn/stellaris/pkg/controller/cluster-set"
	controllerCommon "harmonycloud.cn/stellaris/pkg/controller/common"
	multiClusterRsourceController "harmonycloud.cn/stellaris/pkg/controller/multi-cluster-resource"
	multiPolicyController "harmonycloud.cn/stellaris/pkg/controller/multi-resource-aggregate-policy"
	namespaceMappingController "harmonycloud.cn/stellaris/pkg/controller/namespace-mapping"
	policyController "harmonycloud.cn/stellaris/pkg/controller/resource-aggregate-policy"
	resourceBindingController "harmonycloud.cn/stellaris/pkg/controller/resource-binding"
	resourceSchedulePolicyController "harmonycloud.cn/stellaris/pkg/controller/resource-schedule-policy"
	ruleController "harmonycloud.cn/stellaris/pkg/controller/resource_aggregate_rule"
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
		multiPolicyController.Setup,
		policyController.Setup,
		ruleController.Setup,
	}
	if !args.IsControlPlane {
		controllerSetupFunctions = []func(ctrl.Manager, controllerCommon.Args) error{
			clusterResourceController.Setup,
			policyController.Setup,
			ruleController.Setup,
		}
	}

	for _, setupFunc := range controllerSetupFunctions {
		if err := setupFunc(mgr, args); err != nil {
			return err
		}
	}
	return nil
}
