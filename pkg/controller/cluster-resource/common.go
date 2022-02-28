package cluster_resource

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"harmonycloud.cn/stellaris/pkg/common/helper"

	"harmonycloud.cn/stellaris/config"
	proxy_cfg "harmonycloud.cn/stellaris/pkg/proxy/config"
	proxy_send "harmonycloud.cn/stellaris/pkg/proxy/send"
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

var clusterResourceCommonLog = logf.Log.WithName("proxy_clusterResource_common")

// sync clusterResource when update/create/none
func SyncProxyClusterResource(ctx context.Context, proxyClient *multclusterclient.Clientset, clusterResource *v1alpha1.ClusterResource) error {
	existClusterResource, err := proxyClient.MulticlusterV1alpha1().ClusterResources(clusterResource.GetNamespace()).Get(ctx, clusterResource.Name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			newClusterResourceObject := newClusterResource(clusterResource)
			_, err = proxyClient.MulticlusterV1alpha1().ClusterResources(newClusterResourceObject.GetNamespace()).Create(ctx, newClusterResourceObject, metav1.CreateOptions{})
			return err
		}
		return err
	}
	if reflect.DeepEqual(clusterResource.Spec, existClusterResource.Spec) {
		return nil
	}
	existClusterResource.Spec = clusterResource.Spec
	_, err = proxyClient.MulticlusterV1alpha1().ClusterResources(existClusterResource.GetNamespace()).Update(ctx, existClusterResource, metav1.UpdateOptions{})
	return err
}

func DeleteProxyClusterResource(ctx context.Context, proxyClient *multclusterclient.Clientset, clusterResource *v1alpha1.ClusterResource) error {
	_, err := proxyClient.MulticlusterV1alpha1().ClusterResources(clusterResource.Namespace).Get(ctx, clusterResource.Name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}
	return proxyClient.MulticlusterV1alpha1().ClusterResources(clusterResource.Namespace).Delete(ctx, clusterResource.Name, metav1.DeleteOptions{})
}

// resource compare
func resourceEqual(old, new *unstructured.Unstructured) bool {
	annotationValue, ok := old.GetAnnotations()[managerCommon.ResourceAnnotationKey]
	if !ok {
		return resourceDeepEqual(old.Object, new.Object)
	}
	newAnnotationValue, err := getResourceAnnotations(new)
	if err != nil || len(annotationValue) == 0 {
		return false
	}
	return newAnnotationValue == annotationValue
}

func resourceDeepEqual(old, new map[string]interface{}) bool {
	newSpecObject, ok := new["spec"]
	if !ok {
		return false
	}
	oldSpecObject, ok := old["spec"]
	if !ok {
		return false
	}
	return reflect.DeepEqual(newSpecObject, oldSpecObject)
}

func getUpdateResource(old, new *unstructured.Unstructured) *unstructured.Unstructured {
	_, ok := old.Object["spec"]
	if !ok {
		clusterResourceCommonLog.Info(fmt.Sprintf("can not find old resource(%s:%s) spec", old.GetNamespace(), old.GetName()))
		return changeNoSpecResource(old, new)
	}
	_, ok = new.Object["spec"]
	if !ok {
		clusterResourceCommonLog.Info(fmt.Sprintf("can not find new resource(%s:%s) spec", new.GetNamespace(), new.GetName()))
		delete(old.Object, "spec")
		return changeNoSpecResource(old, new)
	}
	old.Object["spec"] = new.Object["spec"]
	newAnnotationValue, err := getResourceAnnotations(new)
	if err != nil {
		return old
	}
	annotation := old.GetAnnotations()
	annotation[managerCommon.ResourceAnnotationKey] = newAnnotationValue
	old.SetAnnotations(annotation)
	return old
}

func changeNoSpecResource(old, new *unstructured.Unstructured) *unstructured.Unstructured {
	for k, v := range new.Object {
		if k == "apiVersion" || k == "kind" || k == "metadata" || k == "status" {
			continue
		}
		old.Object[k] = v
	}
	return old
}

func createResource(ctx context.Context, clientSet client.Client, resourceObject *unstructured.Unstructured) error {
	annotationValue, _ := getResourceAnnotations(resourceObject)
	if len(annotationValue) > 0 {
		resourceObject.SetAnnotations(map[string]string{
			managerCommon.ResourceAnnotationKey: annotationValue,
		})
	}
	err := clientSet.Create(ctx, resourceObject)
	if apierrors.IsAlreadyExists(err) {
		return nil
	}
	return err
}

func getResourceAnnotations(resource *unstructured.Unstructured) (string, error) {
	specObject, ok := resource.Object["spec"]
	if !ok {
		return "", errors.New("get spec failed")
	}
	specData, err := json.Marshal(specObject)
	if err != nil {
		return "", errors.New("spec json marshal failed")
	}
	return string(specData), nil
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
	request, err := newUpdateClusterResourceStatusRequest([]*v1alpha1.ClusterResource{resource}, proxy_cfg.ProxyConfig.Cfg.ClusterName)
	if err != nil {
		return err
	}
	return proxy_send.SendSyncResourceRequest(request)
}

func newUpdateClusterResourceStatusRequest(clusterResourceList []*v1alpha1.ClusterResource, clusterName string) (*config.Request, error) {
	request := &model.ResourceRequest{}
	for _, clusterResource := range clusterResourceList {
		status := model.ClusterResourceStatus{}
		status.Name = clusterResource.Name
		status.Namespace = managerCommon.ClusterNamespace(clusterName)
		status.Status = clusterResource.Status
		request.ClusterResourceStatusList = append(request.ClusterResourceStatusList, status)
	}
	requestData, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	return proxy_send.NewResourceRequest(model.Resource, clusterName, string(requestData))
}

// send clusterResource to proxy
func sendClusterResourceToProxy(eventType SyncEventType, clusterResource *v1alpha1.ClusterResource) error {
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
	return coreHandler.SendResourceToProxy(clusterName, syncResourceResponse)
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
			return createResource(ctx, clientSet, resourceObject)
		}
		clusterResourceCommonLog.Error(err, fmt.Sprintf("get resource(%s:%s) failed", existResource.GetNamespace(), existResource.GetName()))
		return err
	}

	if resourceEqual(existResource, resourceObject) {
		return nil
	}
	existResource = getUpdateResource(existResource, resourceObject)
	err = clientSet.Update(ctx, existResource)
	if err != nil {
		clusterResourceCommonLog.Error(err, fmt.Sprintf("update resource(%s:%s) failed", existResource.GetNamespace(), existResource.GetName()))
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
	resourceObject, err := helper.GetResourceForRawExtension(instance.Spec.Resource)
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

func newClusterResource(clusterResource *v1alpha1.ClusterResource) *v1alpha1.ClusterResource {
	newClusterResourceObject := &v1alpha1.ClusterResource{}
	newClusterResourceObject.Name = clusterResource.Name
	newClusterResourceObject.Namespace = clusterResource.Namespace
	newClusterResourceObject.Spec = clusterResource.Spec
	return newClusterResourceObject
}
