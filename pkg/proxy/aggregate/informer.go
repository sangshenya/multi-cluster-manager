package aggregate

import (
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"

	proxy_cfg "harmonycloud.cn/stellaris/pkg/proxy/config"

	managerCommon "harmonycloud.cn/stellaris/pkg/common"

	"harmonycloud.cn/stellaris/pkg/apis/multicluster/v1alpha1"
	resourceConfig "harmonycloud.cn/stellaris/pkg/proxy/aggregate/config"
	aggregateController "harmonycloud.cn/stellaris/pkg/proxy/aggregate/controller"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeinformers "k8s.io/client-go/informers"
)

var informerControllerConfigMap *InformerControllerConfigMap

func init() {
	informerControllerConfigMap = &InformerControllerConfigMap{ControllerConfigMap: make(map[string]*InformerControllerConfig)}
}

type InformerControllerConfigMap struct {
	ControllerConfigMap map[string]*InformerControllerConfig
	sync.RWMutex
}

type InformerControllerConfig struct {
	Gvk    *metav1.GroupVersionKind
	stopCh chan struct{}
}

func (i *InformerControllerConfigMap) AddControllerConfig(resourceRef *metav1.GroupVersionKind, controllerStopCh chan struct{}) {
	i.Lock()
	defer i.Unlock()
	i.ControllerConfigMap[managerCommon.GvkLabelString(resourceRef)] = &InformerControllerConfig{
		Gvk:    resourceRef,
		stopCh: controllerStopCh,
	}
}

func (i *InformerControllerConfigMap) RemoveControllerConfig(resourceRef *metav1.GroupVersionKind) {
	i.Lock()
	defer i.Unlock()
	delete(i.ControllerConfigMap, managerCommon.GvkLabelString(resourceRef))
}

func (i *InformerControllerConfigMap) GetControllerConfig(resourceRef *metav1.GroupVersionKind) *InformerControllerConfig {
	i.RLock()
	defer i.RUnlock()
	config, ok := i.ControllerConfigMap[managerCommon.GvkLabelString(resourceRef)]
	if !ok {
		return nil
	}
	return config
}

func RemoveResourceAggregatePolicy(policy *v1alpha1.ResourceAggregatePolicy) error {
	controllerConfig := informerControllerConfigMap.GetControllerConfig(policy.Spec.ResourceRef)
	if controllerConfig == nil {
		return nil
	}

	err := resourceConfig.ResourceConfig.RemoveConfig(policy)
	if err != nil {
		return err
	}
	close(controllerConfig.stopCh)
	informerControllerConfigMap.RemoveControllerConfig(policy.Spec.ResourceRef)
	return nil
}

// add resource config then add informer controller when controller is not found
func AddResourceAggregatePolicy(policy *v1alpha1.ResourceAggregatePolicy) error {
	err := resourceConfig.ResourceConfig.AddConfig(policy)
	if err != nil {
		return err
	}
	controllerConfig := informerControllerConfigMap.GetControllerConfig(policy.Spec.ResourceRef)
	if controllerConfig != nil {
		return nil
	}
	kubeClient, err := kubernetes.NewForConfig(proxy_cfg.ProxyConfig.KubeConfig)
	if err != nil {
		return err
	}
	sharedInformerFactory := kubeinformers.NewSharedInformerFactory(kubeClient, time.Second*30)
	resourceRef := schema.GroupVersionKind{
		Group:   policy.Spec.ResourceRef.Group,
		Version: policy.Spec.ResourceRef.Version,
		Kind:    policy.Spec.ResourceRef.Kind,
	}
	gvrPlural, _ := meta.UnsafeGuessKindToResource(resourceRef)

	informer, err := sharedInformerFactory.ForResource(gvrPlural)
	if err != nil {
		return err
	}
	resourceController, err := aggregateController.NewController(
		policy.Spec.ResourceRef.Kind+"-controller",
		policy.Spec.ResourceRef,
		kubeClient,
		informer,
	)

	if err != nil {
		return err
	}

	stopCh := make(chan struct{})
	go informer.Informer().Run(stopCh)
	go resourceController.Run(2, stopCh)

	informerControllerConfigMap.AddControllerConfig(policy.Spec.ResourceRef, stopCh)
	return nil
}
