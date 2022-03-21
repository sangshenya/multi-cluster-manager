package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	clusterResourceController "harmonycloud.cn/stellaris/pkg/controller/cluster-resource"

	"harmonycloud.cn/stellaris/config"
	"harmonycloud.cn/stellaris/pkg/model"
	proxy_cfg "harmonycloud.cn/stellaris/pkg/proxy/config"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var resourceLog = logf.Log.WithName("proxy_resource")

func RecvSyncResourceResponse(response *config.Response) {
	resourceLog.Info(fmt.Sprintf("recv resource response form core: %s", response.String()))
	switch response.Type {
	case model.ResourceStatusUpdateFailed.String():
		resourceLog.Error(errors.New(response.Body), "cluster resource status update failed")
	case model.ResourceStatusUpdateSuccess.String():
		resourceLog.Info("cluster resource status update success")
	case model.ResourceUpdateOrCreate.String():
		syncClusterResource(response)
	case model.ResourceDelete.String():
		syncClusterResource(response)
	}
}

func syncClusterResource(response *config.Response) {
	resourceRes := &model.SyncResourceResponse{}
	err := json.Unmarshal([]byte(response.Body), resourceRes)
	if err != nil {
		resourceLog.Error(err, fmt.Sprintf("sync proxy(%s) clusterResource failed", response.ClusterName))
		return
	}
	ctx := context.Background()
	for _, clusterResource := range resourceRes.ClusterResourceList {
		resource, err := clusterResourceController.GetClusterResourceObjectForRawExtension(clusterResource)
		if err != nil {
			continue
		}
		clusterResource.SetNamespace(resource.GetNamespace())
		if response.Type == model.ResourceUpdateOrCreate.String() {
			err = clusterResourceController.SyncProxyClusterResource(ctx, proxy_cfg.ProxyConfig.ProxyClient, clusterResource)
			if err != nil {
				resourceLog.Error(err, fmt.Sprintf("updateOrCreate ClusterResource(%s:%s) failed", clusterResource.Namespace, clusterResource.Name))
				continue
			} else {
				resourceLog.Info(fmt.Sprintf("updateOrCreate ClusterResource(%s:%s) success", clusterResource.Namespace, clusterResource.Name))

			}
		} else if response.Type == model.ResourceDelete.String() {
			err = clusterResourceController.DeleteProxyClusterResource(ctx, proxy_cfg.ProxyConfig.ProxyClient, clusterResource)
			if err != nil {
				resourceLog.Error(err, fmt.Sprintf("delete ClusterResource(%s:%s) failed", clusterResource.Namespace, clusterResource.Name))
				continue
			} else {
				resourceLog.Info(fmt.Sprintf("delete ClusterResource(%s:%s) success", clusterResource.Namespace, clusterResource.Name))
			}
		}
	}
}
