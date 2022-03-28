package informers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"

	logf "sigs.k8s.io/controller-runtime/pkg/log"

	utils "harmonycloud.cn/stellaris/pkg/utils/common"

	proxysend "harmonycloud.cn/stellaris/pkg/proxy/send"

	"harmonycloud.cn/stellaris/pkg/proxy/aggregate/match"

	"harmonycloud.cn/stellaris/pkg/apis/multicluster/v1alpha1"
	"harmonycloud.cn/stellaris/pkg/model"

	managerCommon "harmonycloud.cn/stellaris/pkg/common"

	proxy_cfg "harmonycloud.cn/stellaris/pkg/proxy/config"

	"k8s.io/apimachinery/pkg/runtime"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/go-logr/logr"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/informers"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/component-base/metrics/prometheus/ratelimiter"
	"sigs.k8s.io/controller-runtime/pkg/client"

	cueRender "harmonycloud.cn/stellaris/pkg/utils/cue-render"
)

type Controller struct {
	clientSet       clientset.Interface
	eventRecorder   record.EventRecorder
	informer        informers.GenericInformer
	queue           workqueue.RateLimitingInterface
	enqueueResource func(resource client.Object)
	syncHandler     func(key string) error
	resourceRef     *metav1.GroupVersionKind
	controllerName  string
	log             logr.Logger
}

const (
	// maxRetries is the number of times a resource will be retried before it is dropped out of the queue.
	// With the current rate-limiter in use (5ms*2^(maxRetries-1)) the following numbers represent the times
	// a resource is going to be requeued:
	//
	// 5ms, 10ms, 20ms, 40ms, 80ms, 160ms, 320ms, 640ms, 1.3s, 2.6s, 5.1s, 10.2s, 20.4s, 41s, 82s
	maxRetries = 15
)

func NewController(
	controllerName string,
	resourceRef *metav1.GroupVersionKind,
	clientSet clientset.Interface,
	informer informers.GenericInformer) (*Controller, error) {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartStructuredLogging(0)
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: clientSet.CoreV1().Events("")})

	if clientSet != nil && clientSet.CoreV1().RESTClient().GetRateLimiter() != nil {
		if err := ratelimiter.RegisterMetricAndTrackRateLimiterUsage(
			strings.ToLower(resourceRef.Kind)+"_controller",
			clientSet.CoreV1().RESTClient().GetRateLimiter()); err != nil {
			return nil, err
		}
	}

	c := &Controller{
		clientSet:     clientSet,
		eventRecorder: eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: strings.ToLower(resourceRef.Kind) + "-controller"}),
		queue:         workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), strings.ToLower(resourceRef.Kind)),
	}

	c.controllerName = controllerName
	c.resourceRef = resourceRef

	informer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.addResource,
		UpdateFunc: c.updateResource,
		DeleteFunc: c.deleteResource,
	})

	c.informer = informer
	c.syncHandler = c.syncResource
	c.enqueueResource = c.enqueue

	c.log = logf.Log.WithName(strings.ToLower(resourceRef.Kind) + "_controller")

	return c, nil
}

// Run begins watching and syncing.
func (c *Controller) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()
	c.log.Info(fmt.Sprintf("start controller(%s)", c.controllerName))

	if !cache.WaitForNamedCacheSync(strings.ToLower(c.resourceRef.Kind), stopCh, c.informer.Informer().HasSynced) {
		return
	}
	for i := 0; i < workers; i++ {
		go wait.Until(c.worker, time.Second, stopCh)
	}

	<-stopCh
	c.log.Info(fmt.Sprintf("shutting down controller(%s)", c.controllerName))
}

// worker runs a worker thread that just dequeues items, processes them, and marks them done.
// It enforces that the syncHandler is never invoked concurrently with the same key.
func (c *Controller) worker() {
	for c.processNextWorkItem() {
	}
}

func (c *Controller) processNextWorkItem() bool {
	key, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(key)

	err := c.syncHandler(key.(string))
	c.handleErr(err, key)

	return true
}

func (c *Controller) handleErr(err error, key interface{}) {
	if err == nil || apierrors.HasStatusCause(err, v1.NamespaceTerminatingCause) {
		c.queue.Forget(key)
		return
	}

	ns, name, keyErr := cache.SplitMetaNamespaceKey(key.(string))
	if keyErr != nil {
		c.log.Error(err, "Failed to split meta namespace cache key", "cacheKey", key)
	}

	if c.queue.NumRequeues(key) < maxRetries {
		c.log.Info("Error syncing resource", c.controllerName, ns, "/", name, "err", err)
		c.queue.AddRateLimited(key)
		return
	}

	utilruntime.HandleError(err)
	c.log.Info("Dropping deployment out of the queue", c.controllerName, ns, "/", name, "err", err)
	c.queue.Forget(key)
}

// resource enqueue
func (c *Controller) enqueue(resource client.Object) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(resource)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", resource, err))
		return
	}

	c.queue.Add(key)
}

// sync func
func (c *Controller) syncResource(key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		c.log.Error(err, fmt.Sprintf("Failed to split resource(%s) namespace cache key:%s", c.controllerName, key))
		return err
	}

	startTime := time.Now()
	c.log.Info("Started syncing resource", c.controllerName, namespace, "/", name, "startTime", startTime)
	defer func() {
		c.log.Info("Finished syncing resource", c.controllerName, namespace, "/", name, "duration", time.Since(startTime))
	}()

	resource, err := c.informer.Lister().ByNamespace(namespace).Get(name)
	if apierrors.IsNotFound(err) {
		c.log.Info(fmt.Sprintf("resource(%s) has been deleted", key))
		return nil
	}
	if err != nil {
		return err
	}

	unstructuredMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(resource)
	if err != nil {
		return err
	}

	unstructuredResource := &unstructured.Unstructured{}
	unstructuredResource.Object = unstructuredMap
	unstructuredResource.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   c.resourceRef.Group,
		Version: c.resourceRef.Version,
		Kind:    c.resourceRef.Kind,
	})

	ctx := context.Background()
	if !match.IsTargetResource(ctx, c.resourceRef, unstructuredResource) {
		return nil
	}

	// get target resource info
	err = c.aggregateResourceAndSendToCore(ctx, unstructuredResource)
	if err != nil {
		return err
	}
	return nil
}

// event handler
func (c *Controller) addResource(obj interface{}) {
	resource, ok := obj.(client.Object)
	if !ok {
		return
	}
	c.enqueue(resource)
}

func (c *Controller) updateResource(old, cur interface{}) {
	_, ok := old.(client.Object)
	if !ok {
		return
	}
	curD, ok := cur.(client.Object)
	if !ok {
		return
	}
	c.enqueue(curD)
}

func (c *Controller) deleteResource(obj interface{}) {
	resource, ok := obj.(client.Object)
	if !ok {
		return
	}
	c.enqueue(resource)
}

func (c *Controller) aggregateResourceAndSendToCore(ctx context.Context, object client.Object) error {
	data, err := c.getTargetResourceInfo(ctx, object)
	if err != nil {
		return err
	}
	err = c.sendAggregateResourceToCore(data)
	if err != nil {
		return err
	}
	return nil
}

func (c *Controller) getTargetResourceInfo(ctx context.Context, object client.Object) ([]byte, error) {
	// find rule
	var err error
	ruleList, err := utils.GetAggregateRuleListWithLabelSelector(ctx, proxy_cfg.ProxyConfig.ProxyClient, c.resourceRef, metav1.NamespaceAll)
	if err != nil {
		return nil, err
	}

	if len(ruleList.Items) == 0 {
		return nil, errors.New("rule list is empty")
	}

	modelList := &model.AggregateResourceDataModelList{
		List: make([]model.AggregateResourceDataModel, 0, 1),
	}
	for _, rule := range ruleList.Items {
		policyList, err := utils.GetAggregatePolicyListWithLabelSelector(ctx,
			proxy_cfg.ProxyConfig.ProxyClient,
			managerCommon.ManagerNamespace,
			utils.NamespacedName{
				Namespace: rule.GetNamespace(),
				Name:      rule.GetName(),
			})
		if err != nil {
			return nil, err
		}
		resourceData, err := cueRender.RenderCue(object, rule.Spec.Rule.Cue, "")
		if err != nil {
			return nil, err
		}

		for _, policy := range policyList.Items {
			data := NewAggregateResourceDataRequest(&rule, &policy, object, resourceData)
			if data == nil {
				continue
			}
			modelList.List = append(modelList.List, *data)
		}
	}
	data, err := json.Marshal(modelList)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (c *Controller) sendAggregateResourceToCore(data []byte) error {
	request, err := proxysend.NewAggregateRequest(proxy_cfg.ProxyConfig.Cfg.ClusterName, string(data))
	if err != nil {
		return err
	}
	err = proxysend.SendSyncAggregateRequest(request)
	if err != nil {
		return err
	}
	return nil
}

func NewAggregateResourceDataRequest(
	rule *v1alpha1.MultiClusterResourceAggregateRule,
	policy *v1alpha1.ResourceAggregatePolicy,
	object client.Object,
	resourceData []byte) *model.AggregateResourceDataModel {
	data := &model.AggregateResourceDataModel{}
	data.ResourceAggregatePolicy = utils.NamespacedName{
		Namespace: managerCommon.ClusterNamespace(proxy_cfg.ProxyConfig.Cfg.ClusterName),
		Name:      policy.GetName(),
	}
	data.MultiClusterResourceAggregateRule = utils.NamespacedName{
		Namespace: rule.GetNamespace(),
		Name:      rule.GetName(),
	}
	data.ResourceRef = rule.Spec.ResourceRef
	data.TargetResourceData = append(data.TargetResourceData, model.TargetResourceDataModel{
		Namespace: object.GetNamespace(),
		ResourceInfoList: []model.ResourceDataModel{
			{
				Name:         object.GetName(),
				ResourceData: resourceData,
			},
		},
	})
	return data
}
