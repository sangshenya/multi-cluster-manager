package cluster_resource

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

	"k8s.io/klog/v2"

	"harmonycloud.cn/stellaris/pkg/common/helper"

	"harmonycloud.cn/stellaris/config"
	agentconfig "harmonycloud.cn/stellaris/pkg/agent/config"
	agentsend "harmonycloud.cn/stellaris/pkg/agent/send"
	"harmonycloud.cn/stellaris/pkg/apis/multicluster/common"
	"harmonycloud.cn/stellaris/pkg/apis/multicluster/v1alpha1"
	multclusterclient "harmonycloud.cn/stellaris/pkg/client/clientset/versioned"
	managerCommon "harmonycloud.cn/stellaris/pkg/common"
	coreHandler "harmonycloud.cn/stellaris/pkg/core/handler"
	"harmonycloud.cn/stellaris/pkg/model"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// sync clusterResource when update/create/none
func SyncAgentClusterResource(ctx context.Context, agentClient *multclusterclient.Clientset, clusterResource *v1alpha1.ClusterResource) error {
	existClusterResource, err := agentClient.MulticlusterV1alpha1().ClusterResources(clusterResource.GetNamespace()).Get(ctx, clusterResource.Name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			helper.RemoveSurplusParam(clusterResource)
			_, err = agentClient.MulticlusterV1alpha1().ClusterResources(clusterResource.GetNamespace()).Create(ctx, clusterResource, metav1.CreateOptions{})
			return err
		}
		return err
	}
	if reflect.DeepEqual(clusterResource.Spec, existClusterResource.Spec) {
		return syncResource(ctx, agentconfig.AgentConfig.ControllerClient, existClusterResource)
	}
	existClusterResource.Spec = clusterResource.Spec
	_, err = agentClient.MulticlusterV1alpha1().ClusterResources(existClusterResource.GetNamespace()).Update(ctx, existClusterResource, metav1.UpdateOptions{})
	return err
}

func DeleteAgentClusterResource(ctx context.Context, agentClient *multclusterclient.Clientset, clusterResource *v1alpha1.ClusterResource) error {
	_, err := agentClient.MulticlusterV1alpha1().ClusterResources(clusterResource.Namespace).Get(ctx, clusterResource.Name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}
	return agentClient.MulticlusterV1alpha1().ClusterResources(clusterResource.Namespace).Delete(ctx, clusterResource.Name, metav1.DeleteOptions{})
}

func resourceEqual(old, new *unstructured.Unstructured) bool {
	oldSpec, oldOk := old.Object["spec"]
	newSpec, newOk := new.Object["spec"]
	if !(oldOk && newOk) {
		return false
	}
	return reflect.DeepEqual(oldSpec, newSpec)
}

type SyncEventType string

const (
	SyncEventTypeUpdate SyncEventType = "update"
	SyncEventTypeDelete SyncEventType = "delete"
)

// updateClusterResourceStatus send update status request to control plane, then update clusterResource status
func updateClusterResourceStatus(ctx context.Context, clientSet client.Client, clusterResource *v1alpha1.ClusterResource, status v1alpha1.ClusterResourceStatus) error {
	clusterResource.Status = status
	err := sendStatusToControlPlane(clusterResource)
	if err != nil {
		return err
	}
	return clientSet.Status().Update(ctx, clusterResource)
}

// send status to controlPlane
func sendStatusToControlPlane(resource *v1alpha1.ClusterResource) error {
	request, err := newUpdateClusterResourceStatusRequest([]*v1alpha1.ClusterResource{resource}, agentconfig.AgentConfig.Cfg.ClusterName)
	if err != nil {
		return err
	}
	return agentsend.SendSyncResourceRequest(request)
}

func newUpdateClusterResourceStatusRequest(clusterResourceList []*v1alpha1.ClusterResource, clusterName string) (*config.Request, error) {
	request := &model.ResourceRequest{}
	for _, clusterResource := range clusterResourceList {
		status := model.ClusterResourceStatus{}
		status.Name = clusterResource.Name
		status.Namespace = managerCommon.ClusterNamespace(clusterName)
		data, err := json.Marshal(&clusterResource.Status)
		if err != nil {
			return nil, err
		}
		status.Status = string(data)
		request.ClusterResourceStatusList = append(request.ClusterResourceStatusList, status)
	}
	requestData, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	return agentsend.NewResourceRequest(model.Resource, clusterName, string(requestData))
}

// send clusterResource to agent
func sendClusterResourceToAgent(eventType SyncEventType, clusterResource *v1alpha1.ClusterResource) error {
	clusterName := managerCommon.ClusterName(clusterResource.Namespace)
	if len(clusterName) == 0 {
		return errors.New("can not find cluster name")
	}
	resType := model.ResourceUpdateOrCreate
	if eventType == SyncEventTypeDelete {
		resType = model.ResourceDelete
	}
	syncResourceResponse, err := newSyncResourceResponse(resType, clusterName, clusterResource)
	if err != nil {
		return err
	}
	return coreHandler.SendResourceToAgent(clusterName, syncResourceResponse)
}

func newSyncResourceResponse(resType model.ServiceResponseType, clusterName string, clusterResource *v1alpha1.ClusterResource) (*config.Response, error) {
	responseModel := &model.SyncResourceResponse{ClusterResourceList: []*v1alpha1.ClusterResource{clusterResource}}
	data, err := json.Marshal(responseModel)
	if err != nil {
		return nil, err
	}
	return coreHandler.NewResourceResponse(resType, clusterName, string(data))
}

// sync Resource when create/update/delete
// syncResource create or update resource
func syncResource(ctx context.Context, clientSet client.Client, instance *v1alpha1.ClusterResource) error {
	resourceObject, err := GetClusterResourceObjectForRawExtension(instance)
	if err != nil {
		return err
	}
	existResource := &unstructured.Unstructured{}
	existResource.SetGroupVersionKind(resourceObject.GroupVersionKind())
	err = clientSet.Get(ctx, types.NamespacedName{
		Namespace: resourceObject.GetNamespace(),
		Name:      resourceObject.GetName(),
	}, existResource)

	if err != nil {
		if apierrors.IsNotFound(err) {
			err = clientSet.Create(ctx, resourceObject)
			if apierrors.IsAlreadyExists(err) {
				return nil
			}
			return err
		}
		klog.V(1).ErrorS(err, fmt.Sprintf("get resource(%s:%s) failed", existResource.GetNamespace(), existResource.GetName()))
		return err
	}

	if resourceEqual(existResource, resourceObject) {
		return nil
	}
	existResource.Object["spec"] = resourceObject.Object["spec"]

	err = clientSet.Update(ctx, existResource)
	if err != nil {
		klog.V(1).ErrorS(err, fmt.Sprintf("update resource(%s:%s) failed", existResource.GetNamespace(), existResource.GetName()))
	}
	return err
}

func deleteResource(ctx context.Context, clientSet client.Client, instance *v1alpha1.ClusterResource) error {
	resourceObject, err := GetClusterResourceObjectForRawExtension(instance)
	if err != nil {
		return err
	}

	err = clientSet.Delete(ctx, resourceObject)
	if apierrors.IsNotFound(err) {
		return nil
	}
	return err
}

func GetClusterResourceObjectForRawExtension(instance *v1alpha1.ClusterResource) (*unstructured.Unstructured, error) {
	if instance.Spec.Resource == nil {
		return nil, errors.New("resource is empty")
	}
	resourceByte, err := instance.Spec.Resource.MarshalJSON()
	if err != nil {
		return nil, err
	}
	resourceObject := &unstructured.Unstructured{}
	err = resourceObject.UnmarshalJSON(resourceByte)
	if err != nil {
		return nil, err
	}
	owner := metav1.NewControllerRef(instance, v1alpha1.ClusterResourceGroupVersionKind)
	resourceObject.SetOwnerReferences([]metav1.OwnerReference{*owner})
	return resourceObject, err
}

// clusterResource status
func newClusterResourceStatus(phase common.MultiClusterResourcePhase, message string, generation int64) v1alpha1.ClusterResourceStatus {
	return v1alpha1.ClusterResourceStatus{
		ObservedReceiveGeneration: generation,
		Phase:                     phase,
		Message:                   message,
	}
}
