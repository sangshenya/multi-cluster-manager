package resource_binding

import (
	"context"
	"strings"

	"k8s.io/apimachinery/pkg/labels"

	"harmonycloud.cn/stellaris/pkg/apis/multicluster/common"
	"harmonycloud.cn/stellaris/pkg/apis/multicluster/v1alpha1"
	managerCommon "harmonycloud.cn/stellaris/pkg/common"
	controllerCommon "harmonycloud.cn/stellaris/pkg/controller/common"
	sliceutil "harmonycloud.cn/stellaris/pkg/utils/slice"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func SyncClusterResourceWithBinding(ctx context.Context, clientSet client.Client, binding *v1alpha1.MultiClusterResourceBinding) error {
	// get ClusterResourceList
	clusterResourceList, err := getClusterResourceListForBinding(ctx, clientSet, binding)
	if err != nil {
		return err
	}
	return syncClusterResource(ctx, clientSet, clusterResourceList, binding)
}

// syncClusterResource update or create or delete ClusterResource
func syncClusterResource(ctx context.Context, clientSet client.Client, clusterResourceList *v1alpha1.ClusterResourceList, binding *v1alpha1.MultiClusterResourceBinding) error {
	if len(binding.Spec.Resources) == 0 {
		return nil
	}

	clusterResourceMap := changeClusterResourceListToMap(clusterResourceList)
	for _, resource := range binding.Spec.Resources {
		for _, cluster := range resource.Clusters {
			multiClusterResource, err := getMultiClusterResourceForName(ctx, clientSet, resource.Name, resource.Namespace)
			if err != nil {
				continue
			}

			key := mapKey(managerCommon.ClusterNamespace(cluster.Name), getClusterResourceName(binding.Name, multiClusterResource.Spec.ResourceRef))
			clusterResource, ok := clusterResourceMap[key]
			if !ok {
				// new clusterResource
				owner := metav1.NewControllerRef(binding, v1alpha1.MultiClusterResourceBindingGroupVersionKind)
				clusterResource = newClusterResource(binding.Name, cluster, owner, multiClusterResource)

				// create clusterResource
				err = clientSet.Create(ctx, clusterResource)
				if err != nil {
					return err
				}
			} else {
				delete(clusterResourceMap, key)
				// new resourceInfo
				// TODO if MultiClusterResourceOverride alive
				resourceInfo, err := controllerCommon.ApplyJsonPatch(multiClusterResource.Spec.Resource, cluster.Override)
				if err != nil {
					resourceInfo = multiClusterResource.Spec.Resource
				}
				if string(clusterResource.Spec.Resource.Raw) == string(resourceInfo.Raw) {
					continue
				}
				// update
				clusterResource.Spec.Resource = resourceInfo
				err = clientSet.Update(ctx, clusterResource)
				if err != nil {
					return err
				}
			}
		}
	}

	if len(clusterResourceMap) <= 0 {
		return nil
	}

	// delete
	for _, r := range clusterResourceMap {
		err := clientSet.Delete(ctx, r)
		if err != nil {
			return err
		}
	}
	return nil
}

func newClusterResource(bindingName string, cluster v1alpha1.MultiClusterResourceBindingCluster, owner *metav1.OwnerReference, multiClusterResource *v1alpha1.MultiClusterResource) *v1alpha1.ClusterResource {
	clusterNamespace := managerCommon.ClusterNamespace(cluster.Name)
	clusterResourceName := getClusterResourceName(bindingName, multiClusterResource.Spec.ResourceRef)

	clusterResource := &v1alpha1.ClusterResource{}
	clusterResource.SetName(clusterResourceName)
	clusterResource.SetNamespace(clusterNamespace)
	// set labels
	newLabels := clusterResourceLabels(bindingName, multiClusterResource.GetName(), multiClusterResource.Spec.ResourceRef)
	clusterResource.SetLabels(newLabels)
	// set owner
	clusterResource.SetOwnerReferences([]metav1.OwnerReference{*owner})

	// set resourceInfo
	// TODO if MultiClusterResourceOverride alive
	resourceInfo, err := controllerCommon.ApplyJsonPatch(multiClusterResource.Spec.Resource, cluster.Override)
	if err == nil {
		clusterResource.Spec.Resource = resourceInfo
	}
	return clusterResource
}

func clusterResourceLabels(bindingName, multiClusterResourceName string, multiClusterResourceRef *metav1.GroupVersionKind) map[string]string {
	newLabels := map[string]string{}
	newLabels[managerCommon.ResourceBindingLabelName] = bindingName
	newLabels[managerCommon.ResourceGvkLabelName] = managerCommon.GvkLabelString(multiClusterResourceRef)
	newLabels[managerCommon.MultiClusterResourceLabelName] = multiClusterResourceName
	return newLabels
}

func removeItemForClusterStatusList(itemList []common.MultiClusterResourceClusterStatus, item common.MultiClusterResourceClusterStatus) []common.MultiClusterResourceClusterStatus {
	if len(itemList) <= 0 {
		return itemList
	}
	var objectList []interface{}
	for _, items := range itemList {
		objectList = append(objectList, items)
	}

	index := sliceutil.GetIndexWithObject(objectList, item)
	list := sliceutil.RemoveObjectWithIndex(objectList, index)
	if len(list) == 0 {
		return []common.MultiClusterResourceClusterStatus{}
	}
	var statusList []common.MultiClusterResourceClusterStatus
	for _, obj := range list {
		if status, ok := obj.(common.MultiClusterResourceClusterStatus); ok {
			statusList = append(statusList, status)
		}
	}
	return statusList
}

func getClusterResourceName(bindingName string, gvk *metav1.GroupVersionKind) string {
	gvkString := managerCommon.GvkLabelString(gvk)
	return bindingName + "." + gvkString
}

func mapKey(resourceNamespace, resourceName string) string {
	return resourceNamespace + "." + resourceName
}

// getClusterResourceListForBinding change clusterResource list to map
// clusterResource list change to clusterResource map, map key:<resourceNamespace>-<resourceName>
func changeClusterResourceListToMap(resourceList *v1alpha1.ClusterResourceList) map[string]*v1alpha1.ClusterResource {
	resourceMap := map[string]*v1alpha1.ClusterResource{}
	for _, resource := range resourceList.Items {
		if !strings.HasPrefix(resource.GetNamespace(), managerCommon.ClusterWorkspacePrefix) {
			continue
		}
		key := mapKey(resource.GetNamespace(), resource.GetName())
		resourceMap[key] = &resource
	}
	return resourceMap
}

func getClusterResourceListForBinding(ctx context.Context, clientSet client.Client, binding *v1alpha1.MultiClusterResourceBinding) (*v1alpha1.ClusterResourceList, error) {
	selector, _ := labels.Parse(managerCommon.ResourceBindingLabelName + "=" + binding.GetName())

	resourceList := &v1alpha1.ClusterResourceList{}
	err := clientSet.List(ctx, resourceList, &client.ListOptions{
		LabelSelector: selector,
	})
	return resourceList, err
}
