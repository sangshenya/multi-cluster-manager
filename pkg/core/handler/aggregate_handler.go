package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"harmonycloud.cn/stellaris/pkg/common/helper"

	"k8s.io/apimachinery/pkg/runtime"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	managerCommon "harmonycloud.cn/stellaris/pkg/common"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"harmonycloud.cn/stellaris/pkg/apis/multicluster/v1alpha1"

	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"harmonycloud.cn/stellaris/pkg/model"
	coreUtils "harmonycloud.cn/stellaris/pkg/utils/core"

	"harmonycloud.cn/stellaris/config"
)

var coreAggregateLog = logf.Log.WithName("core_aggregate")

func (s *CoreServer) Aggregate(req *config.Request, stream config.Channel_EstablishServer) {
	resourceHandlerLog.Info(fmt.Sprintf("receive grpc request for aggregate, cluster:%s", req.ClusterName))
	requestModel := &model.AggregateResourceDataModelList{}
	err := json.Unmarshal([]byte(req.Body), requestModel)
	if err != nil {
		coreAggregateLog.Error(err, "unmarshal data error")
		coreUtils.SendErrResponse(req.ClusterName, model.AggregateResourceFailed, err, stream)
	}
	coreAggregateLog.Info("aggregate resource data")
	ctx := context.Background()
	err = s.aggregateResourceInfo(ctx, requestModel, req.ClusterName)
	if err != nil {
		coreAggregateLog.Error(err, "aggregate resource info failed")
		coreUtils.SendErrResponse(req.ClusterName, model.AggregateResourceFailed, err, stream)
		return
	}
	coreAggregateLog.Info("aggregate resource info success")

	res := &config.Response{
		Type:        model.AggregateResourceSuccess.String(),
		ClusterName: req.ClusterName,
		Body:        req.Body,
	}
	coreUtils.SendResponse(res, stream)
}

func (s *CoreServer) aggregateResourceInfo(ctx context.Context, modelList *model.AggregateResourceDataModelList, clusterName string) error {
	for _, modelItem := range modelList.List {
		err := s.syncAggregateResource(ctx, clusterName, &modelItem)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *CoreServer) syncAggregateResource(ctx context.Context, clusterName string, resourceData *model.AggregateResourceDataModel) error {
	for _, resource := range resourceData.TargetResourceData {
		mappingNs, err := helper.GetNamespaceMapping(ctx, s.clientSet, clusterName, resource.Namespace)
		if err != nil {
			return err
		}
		resource.Namespace = mappingNs
		isAlive, err := validateNamespace(ctx, s.clientSet, resource.Namespace)
		if err != nil {
			return err
		}
		// get namespace and AggregatedResource
		aggregateResource := &v1alpha1.AggregatedResource{}
		aggregateResourceName := getAggregateResourceName(resourceData.ResourceRef)
		if !isAlive {
			// should create namespace
			err = createNamespace(ctx, s.clientSet, resource.Namespace)
			if err != nil {
				return err
			}
		} else {
			err = s.clientSet.Get(ctx, types.NamespacedName{
				Namespace: resource.Namespace,
				Name:      aggregateResourceName,
			}, aggregateResource)
			if err != nil && !apierrors.IsNotFound(err) {
				return err
			}
		}

		// should create AggregateResource
		if len(aggregateResource.GetName()) == 0 {
			parentResourceNamespace, err := helper.GetNamespaceMapping(ctx, s.clientSet, clusterName, resourceData.MultiClusterResourceAggregateRule.Namespace)
			if err != nil {
				return err
			}

			aggregateResource, err = newAggregateResource(clusterName, parentResourceNamespace, resource, resourceData)
			if err != nil {
				return err
			}
			err = s.clientSet.Create(ctx, aggregateResource)
			if err != nil {
				return err
			}
			continue
		}
		// should update AggregateResource
		aggregateResource = validateAggregatedResource(aggregateResource, clusterName, resource)
		if err = s.clientSet.Update(ctx, aggregateResource); err != nil {
			return err
		}
	}
	return nil
}

func validateAggregatedResource(
	aggregateResource *v1alpha1.AggregatedResource,
	clusterName string,
	resource model.TargetResourceDataModel) *v1alpha1.AggregatedResource {
	for index, cluster := range aggregateResource.Clusters {
		if cluster.Name != clusterName || cluster.ResourceNamespace != resource.Namespace {
			continue
		}
		aggregateResource.Clusters[index] = v1alpha1.AggregatedResourceClusters{
			Name:              clusterName,
			ResourceNamespace: resource.Namespace,
			ResourceList:      coreUtils.ConvertResourceInfoList2KubeResourceInfoList(resource.ResourceInfoList),
		}
		return aggregateResource
	}
	aggregateResource.Clusters = append(aggregateResource.Clusters, v1alpha1.AggregatedResourceClusters{
		Name:              clusterName,
		ResourceNamespace: resource.Namespace,
		ResourceList:      coreUtils.ConvertResourceInfoList2KubeResourceInfoList(resource.ResourceInfoList),
	})
	return aggregateResource
}

//func resourceInfoEqual(old []v1alpha1.ResourceInfo, new []model.ResourceDataModel) bool {
//	if len(old) != len(new) {
//		return false
//	}
//	for _, oldItem := range old {
//		equalName := false
//		for _, newItem := range new {
//			if oldItem.ResourceName == newItem.Name {
//				equalName = true
//				if !reflect.DeepEqual(oldItem.Result.Raw, newItem.ResourceData) {
//					return false
//				}
//
//			}
//		}
//		if !equalName {
//			return false
//		}
//	}
//	return true
//}

func getAggregateResourceLabels(parentResourceNamespace, ruleName, policyName string, resourceRef *metav1.GroupVersionKind) map[string]string {
	labels := map[string]string{}
	labels[managerCommon.AggregateRuleLabelName] = ruleName
	labels[managerCommon.AggregatePolicyLabelName] = policyName
	labels[managerCommon.AggregateResourceGvkLabelName] = managerCommon.GvkLabelString(resourceRef)
	labels[managerCommon.ParentResourceNamespaceLabelName] = parentResourceNamespace
	return labels
}

func newAggregateResource(
	clusterName,
	parentNamespace string,
	resourceInfo model.TargetResourceDataModel,
	model *model.AggregateResourceDataModel) (*v1alpha1.AggregatedResource, error) {
	if len(resourceInfo.ResourceInfoList) == 0 {
		return nil, errors.New("resource info list is empty")
	}
	aggregateResource := &v1alpha1.AggregatedResource{}
	aggregateResource.SetName(getAggregateResourceName(model.ResourceRef))
	aggregateResource.SetNamespace(resourceInfo.Namespace)
	aggregatedResourceClusters := v1alpha1.AggregatedResourceClusters{
		Name:              clusterName,
		ResourceNamespace: resourceInfo.Namespace,
	}
	// add labels
	labels := getAggregateResourceLabels(
		parentNamespace,
		model.MultiClusterResourceAggregateRule.Name,
		model.ResourceAggregatePolicy.Name,
		model.ResourceRef)
	aggregateResource.SetLabels(labels)

	for _, item := range resourceInfo.ResourceInfoList {
		if len(item.Name) == 0 || len(item.ResourceData) == 0 {
			coreAggregateLog.Info("resource info or name is empty")
			continue
		}
		var info v1alpha1.ResourceInfo
		info.ResourceName = item.Name
		info.Result = runtime.RawExtension{
			Raw: item.ResourceData,
		}
		aggregatedResourceClusters.ResourceList = append(aggregatedResourceClusters.ResourceList, info)
	}
	return aggregateResource, nil
}

func getAggregateResourceName(gvk *metav1.GroupVersionKind) string {
	return managerCommon.GvkLabelString(gvk)
}

func validateNamespace(ctx context.Context, clientSet client.Client, namespace string) (bool, error) {
	ns := &v1.Namespace{}
	err := clientSet.Get(ctx, types.NamespacedName{
		Namespace: "",
		Name:      namespace,
	}, ns)
	if err != nil {
		return false, err
	}
	return true, nil
}

func createNamespace(ctx context.Context, clientSet client.Client, namespace string) error {
	ns := &v1.Namespace{}
	ns.SetName(namespace)
	return clientSet.Create(ctx, ns)
}
