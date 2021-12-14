package resource_binding

import (
	"context"
	"errors"

	"k8s.io/apimachinery/pkg/labels"

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
	if controllerRef == nil {
		return errors.New("can not find binding")
	}
	binding := resolveControllerRef(clientSet, controllerRef)
	if binding == nil {
		return errors.New("can not find binding")
	}

	clusterName := managerCommon.ClusterName(clusterResource.Namespace)

	var resourceStatus *common.MultiClusterResourceClusterStatus
	for _, item := range binding.Status.ClusterStatus {
		if clusterName == item.Name && clusterResource.Name == item.Resource {
			if statusEqual(clusterResource.Status, item) {
				return nil
			}
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
	// update binding status
	err := clientSet.Status().Update(context.TODO(), binding)
	return err
}

func statusEqual(clusterResourceStatus v1alpha1.ClusterResourceStatus, bindingStatus common.MultiClusterResourceClusterStatus) bool {
	if clusterResourceStatus.Phase != bindingStatus.Phase || clusterResourceStatus.Message != bindingStatus.Message || clusterResourceStatus.ObservedReceiveGeneration != bindingStatus.ObservedReceiveGeneration {
		return false
	}
	return true
}

func resolveControllerRef(clientSet client.Client, controllerRef *metav1.OwnerReference) *v1alpha1.MultiClusterResourceBinding {
	if controllerRef.Kind != "MultiClusterResourceBinding" {
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

// syncClusterResource update or create or delete ClusterResource
func syncClusterResource(clientSet client.Client, binding *v1alpha1.MultiClusterResourceBinding) error {
	if len(binding.Spec.Resources) == 0 {
		return nil
	}

	clusterResourceMap, err := getClusterResourceListForBinding(clientSet, binding)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
	}

	for _, resource := range binding.Spec.Resources {
		for _, cluster := range resource.Clusters {
			multiClusterResource, err := getMultiClusterResourceForName(clientSet, resource.Name)
			if err != nil {
				continue
			}

			key := mapKey(managerCommon.ClusterNamespace(cluster.Name), getClusterResourceName(binding.Name, multiClusterResource.Spec.ResourceRef))
			clusterResource, ok := clusterResourceMap[key]
			if !ok {
				// new clusterResource
				owner := metav1.NewControllerRef(binding, binding.GroupVersionKind())
				clusterResource = newClusterResource(binding.Name, cluster, owner, multiClusterResource)

				// create clusterResource
				err = clientSet.Create(context.TODO(), clusterResource)
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
				err = clientSet.Update(context.TODO(), clusterResource)
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
		err = clientSet.Delete(context.TODO(), r)
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
	newLabels := clusterResourceLabels(bindingName, multiClusterResource.Spec.ResourceRef)
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

func clusterResourceLabels(bindingName string, multiClusterResourceRef *metav1.GroupVersionKind) map[string]string {
	newLabels := map[string]string{}
	newLabels[managerCommon.ResourceBindingLabelName] = bindingName
	newLabels[managerCommon.ResourceGvkLabelName] = managerCommon.GvkLabelString(multiClusterResourceRef)
	return newLabels
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

func getClusterResourceName(bindingName string, gvk *metav1.GroupVersionKind) string {
	gvkString := managerCommon.GvkLabelString(gvk)
	return bindingName + gvkString
}

func mapKey(resourceNamespace, resourceName string) string {
	return resourceNamespace + ":" + resourceName
}

// getClusterResourceListForBinding go through ResourceBinding to find clusterResource list
// clusterResource list change to clusterResource map, map key:<resourceNamespace>-<resourceName>
func getClusterResourceListForBinding(clientSet client.Client, binding *v1alpha1.MultiClusterResourceBinding) (map[string]*v1alpha1.ClusterResource, error) {
	if len(binding.GetName()) <= 0 {
		return nil, errors.New("binding name is empty")
	}
	resourceMap := map[string]*v1alpha1.ClusterResource{}
	selector, _ := labels.Parse(managerCommon.ResourceBindingLabelName + "=" + binding.GetName())

	resourceList := &v1alpha1.ClusterResourceList{}
	err := clientSet.List(context.TODO(), resourceList, &client.ListOptions{
		LabelSelector: selector,
	})
	if err != nil {
		return resourceMap, err
	}
	for _, resource := range resourceList.Items {
		key := mapKey(resource.GetNamespace(), resource.GetName())
		resourceMap[key] = &resource
	}
	return resourceMap, nil
}
