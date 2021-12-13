package resource_binding

import (
	"context"
	"errors"

	"harmonycloud.cn/stellaris/pkg/apis/multicluster/common"
	"harmonycloud.cn/stellaris/pkg/apis/multicluster/v1alpha1"
	managerCommon "harmonycloud.cn/stellaris/pkg/common"
	controllerCommon "harmonycloud.cn/stellaris/pkg/controller/common"
	sliceutil "harmonycloud.cn/stellaris/pkg/util/slice"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func updateBindingStatus(clientSet client.Client, clusterResource *v1alpha1.ClusterResource) error {
	if len(clusterResource.Status.Phase) <= 0 {
		return nil
	}
	controllerRef := metav1.GetControllerOf(clusterResource)
	binding := resolveControllerRef(clientSet, controllerRef)

	//
	clusterName := managerCommon.ClusterName(clusterResource.Namespace)
	// update
	var resourceStatus *common.MultiClusterResourceClusterStatus
	for _, item := range binding.Status.ClusterStatus {
		if clusterName == item.Name {
			// delete
			binding.Status.ClusterStatus = removeItemForClusterStatusList(binding.Status.ClusterStatus, item)
		}
	}
	resourceStatus = &common.MultiClusterResourceClusterStatus{
		Name:                      clusterName,
		Resource:                  clusterResource.Name,
		ObservedReceiveGeneration: clusterResource.Status.ObservedReceiveGeneration,
		Phase:                     clusterResource.Status.Phase,
		Message:                   clusterResource.Status.Message,
		Binding:                   binding.Name,
	}
	binding.Status.ClusterStatus = append(binding.Status.ClusterStatus, *resourceStatus)

	//
	err := clientSet.Update(context.TODO(), binding)
	return err
}

func resolveControllerRef(clientSet client.Client, controllerRef *metav1.OwnerReference) *v1alpha1.MultiClusterResourceBinding {
	if controllerRef == nil || controllerRef.Kind != "MultiClusterResourceBinding" {
		return nil
	}
	binding := &v1alpha1.MultiClusterResourceBinding{}
	namespacedName := types.NamespacedName{
		Namespace: managerCommon.ManagerNamespace,
		Name:      controllerRef.Name,
	}
	err := clientSet.Get(context.TODO(), namespacedName, binding)
	if err != nil {
		return nil
	}
	if binding.UID != controllerRef.UID {
		return nil
	}
	return binding
}

func syncClusterResource(clientSet client.Client, binding *v1alpha1.MultiClusterResourceBinding) error {
	if len(binding.Spec.Resources) == 0 {
		return nil
	}
	owner := metav1.NewControllerRef(binding, binding.GroupVersionKind())
	if owner == nil {
		return errors.New("get owner fail")
	}

	//
	clusterResourceMap, err := getClusterResourceListForBinding(clientSet, binding)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
	}

	for _, resource := range binding.Spec.Resources {
		if resource.Clusters == nil {
			continue
		}
		for _, cluster := range resource.Clusters {
			multiClusterResource, err := getMultiClusterResourceForName(clientSet, resource.Name)
			if err != nil {
				continue
			}

			clusterNamespace := managerCommon.ClusterNamespace(cluster.Name)
			clusterResourceName := getClusterResourceName(binding.Name, multiClusterResource.Spec.ResourceRef)
			key := mapKey(clusterNamespace, clusterResourceName)

			clusterResource, ok := clusterResourceMap[key]
			if !ok {
				// create
				newClusterResource := &v1alpha1.ClusterResource{}
				newClusterResource.SetName(clusterResourceName)
				newClusterResource.SetNamespace(clusterNamespace)
				// labels
				newLabels := map[string]string{}
				newLabels[managerCommon.ResourceBindingLabelName] = binding.GetName()
				//
				newLabels[managerCommon.ResourceGvkLabelName] = managerCommon.GvkLabelString(multiClusterResource.Spec.ResourceRef)
				newClusterResource.SetLabels(newLabels)
				// OwnerReferences
				newClusterResource.SetOwnerReferences([]metav1.OwnerReference{*owner})
				// resourceInfo
				if cluster.Override != nil {
					resourceInfo, err := controllerCommon.ApplyJsonPatch(multiClusterResource.Spec.Resource, cluster.Override)
					if err == nil {
						newClusterResource.Spec.Resource = resourceInfo
					}
				} else {
					newClusterResource.Spec.Resource = multiClusterResource.Spec.Resource
				}
				// create clusterResource
				err = clientSet.Create(context.TODO(), newClusterResource)
				if err != nil {
					return err
				}
			} else {
				// update
				// resourceInfo
				resourceInfo := multiClusterResource.Spec.Resource
				if cluster.Override != nil {
					rInfo, err := controllerCommon.ApplyJsonPatch(multiClusterResource.Spec.Resource, cluster.Override)
					if err == nil {
						resourceInfo = rInfo
					}
				}
				if string(clusterResource.Spec.Resource.Raw) != string(resourceInfo.Raw) {
					clusterResource.Spec.Resource = resourceInfo
					// labels
					newLabels := clusterResource.GetLabels()
					newLabels[managerCommon.ResourceBindingLabelName] = binding.GetName()
					//
					newLabels[managerCommon.ResourceGvkLabelName] = managerCommon.GvkLabelString(multiClusterResource.Spec.ResourceRef)
					clusterResource.SetLabels(newLabels)
					// OwnerReferences
					clusterResource.SetOwnerReferences([]metav1.OwnerReference{*owner})
					// update
					err = clientSet.Update(context.TODO(), clusterResource)
					if err != nil {
						return err
					}
				}
				delete(clusterResourceMap, key)
			}
		}
	}

	if len(clusterResourceMap) <= 0 {
		return nil
	}

	// delete
	for _, r := range clusterResourceMap {
		err = clientSet.Delete(context.TODO(), r)
		if err != nil {
			return err
		}
	}
	return nil
}

func removeItemForClusterStatusList(itemList []common.MultiClusterResourceClusterStatus, item common.MultiClusterResourceClusterStatus) []common.MultiClusterResourceClusterStatus {
	var objectList []interface{}
	for _, items := range itemList {
		objectList = append(objectList, items)
	}

	index := sliceutil.GetIndexWithObject(objectList, item)
	list := sliceutil.RemoveObjectWithIndex(objectList, index)
	if len(list) <= 0 {
		return itemList
	}
	var statusList []common.MultiClusterResourceClusterStatus
	for _, obj := range list {
		if status, ok := obj.(common.MultiClusterResourceClusterStatus); ok {
			statusList = append(statusList, status)
		}
	}
	return statusList
}
