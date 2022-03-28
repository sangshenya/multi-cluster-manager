package webhook

import (
	controllerCommon "harmonycloud.cn/stellaris/pkg/controller/common"
	"harmonycloud.cn/stellaris/pkg/webhook/cluster"
	cluster_resource "harmonycloud.cn/stellaris/pkg/webhook/cluster-resource"
	cluster_resource_aggregate_rule "harmonycloud.cn/stellaris/pkg/webhook/cluster-resource-aggregate-rule"
	multi_cluster_resource "harmonycloud.cn/stellaris/pkg/webhook/multi-cluster-resource"
	resource_schedule_policy "harmonycloud.cn/stellaris/pkg/webhook/resource-schedule-policy"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// Register will be called in main and register all validation handlers
func Register(mgr manager.Manager, args controllerCommon.Args) {
	server := mgr.GetWebhookServer()
	server.Register("/validating-v1alpha1-cluster",
		&webhook.Admission{Handler: &cluster.ValidatingAdmission{}})
	server.Register("/validating-v1alpha1-clusterresource",
		&webhook.Admission{Handler: &cluster_resource.ValidatingAdmission{}})
	server.Register("/validating-v1alpha1-multiclusterresourceaggregaterules",
		&webhook.Admission{Handler: &cluster_resource_aggregate_rule.ValidatingAdmission{}})
	server.Register("/validating-v1alpha1-multiclusterresource",
		&webhook.Admission{Handler: &multi_cluster_resource.ValidatingAdmission{}})
	server.Register("/validating-v1alpha1-multiclusterresourceschedulepolicy",
		&webhook.Admission{Handler: &resource_schedule_policy.ValidatingAdmission{}})

}
