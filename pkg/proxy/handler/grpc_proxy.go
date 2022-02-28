package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	resource_aggregate_policy "harmonycloud.cn/stellaris/pkg/controller/resource-aggregate-policy"
	"harmonycloud.cn/stellaris/pkg/controller/resource_aggregate_rule"

	"harmonycloud.cn/stellaris/pkg/proxy/send"

	clusterResourceController "harmonycloud.cn/stellaris/pkg/controller/cluster-resource"

	"harmonycloud.cn/stellaris/config"
	"harmonycloud.cn/stellaris/pkg/apis/multicluster/v1alpha1"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	multclusterclient "harmonycloud.cn/stellaris/pkg/client/clientset/versioned"
	"harmonycloud.cn/stellaris/pkg/model"
	proxy_cfg "harmonycloud.cn/stellaris/pkg/proxy/config"
)

var registerLog = logf.Log.WithName("proxy_register")

func RecvRegisterResponse(response *config.Response) {
	registerLog.Info(fmt.Sprintf("get response from server:%s", response.String()))

	var err error
	if response.Type != model.RegisterSuccess.String() {
		err = errors.New(response.Body)
		registerLog.Error(err, "response is not register success")
		return
	}

	registerLog.Info(fmt.Sprintf("start send heartbeat"))
	go send.HeartbeatStart()

	err = dealResponse(proxy_cfg.ProxyConfig.ProxyClient, response)
	if err != nil {
		registerLog.Error(err, "deal response failed")
	}
}

func dealResponse(proxyClient *multclusterclient.Clientset, response *config.Response) error {
	if len(response.Body) <= 0 {
		return nil
	}
	resources := &model.RegisterResponse{}
	err := json.Unmarshal([]byte(response.Body), resources)
	if err != nil {
		return err
	}
	return syncResource(proxyClient, resources)
}

func syncResource(proxyClient *multclusterclient.Clientset, resourceList *model.RegisterResponse) error {
	ctx := context.Background()
	if err := syncClusterResourcesList(ctx, proxyClient, resourceList.ClusterResources); err != nil {
		return err
	}
	if err := resource_aggregate_policy.SyncAggregatePolicyList(ctx, proxyClient, model.SyncResource, resourceList.ResourceAggregatePolicies); err != nil {
		return err
	}

	if err := resource_aggregate_rule.SyncAggregateRuleList(ctx, proxyClient, model.SyncResource, resourceList.MultiClusterResourceAggregateRules); err != nil {
		return err
	}
	return nil
}

func syncClusterResourcesList(ctx context.Context, proxyClient *multclusterclient.Clientset, clusterResourceList []v1alpha1.ClusterResource) error {
	for _, clusterResource := range clusterResourceList {
		resource, err := clusterResourceController.GetClusterResourceObjectForRawExtension(&clusterResource)
		if err != nil {
			continue
		}
		clusterResource.SetNamespace(resource.GetNamespace())
		err = clusterResourceController.SyncProxyClusterResource(ctx, proxyClient, &clusterResource)
		if err != nil {
			registerLog.Error(err, fmt.Sprintf("sync ClusterResource(%s:%s) failed", clusterResource.Namespace, clusterResource.Name))
			return err
		} else {
			registerLog.Info(fmt.Sprintf("sync ClusterResource(%s:%s) success", clusterResource.Namespace, clusterResource.Name))
		}
	}
	return nil
}
