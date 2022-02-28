package informers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"harmonycloud.cn/stellaris/pkg/proxy/aggregate/match"

	proxysend "harmonycloud.cn/stellaris/pkg/proxy/send"

	"harmonycloud.cn/stellaris/pkg/apis/multicluster/v1alpha1"
	"harmonycloud.cn/stellaris/pkg/model"

	managerCommon "harmonycloud.cn/stellaris/pkg/common"
	"harmonycloud.cn/stellaris/pkg/util/common"

	resource_aggregate_policy "harmonycloud.cn/stellaris/pkg/controller/resource-aggregate-policy"

	"harmonycloud.cn/stellaris/pkg/controller/resource_aggregate_rule"
	proxy_cfg "harmonycloud.cn/stellaris/pkg/proxy/config"

	"k8s.io/apimachinery/pkg/runtime"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/klog/v2"

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

func NewController(controllerName string, resourceRef *metav1.GroupVersionKind, clientSet clientset.Interface, informer informers.GenericInformer) (*Controller, error) {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartStructuredLogging(0)
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: clientSet.CoreV1().Events("")})

	if clientSet != nil && clientSet.CoreV1().RESTClient().GetRateLimiter() != nil {
		if err := ratelimiter.RegisterMetricAndTrackRateLimiterUsage(controllerName, clientSet.CoreV1().RESTClient().GetRateLimiter()); err != nil {
			return nil, err
		}
	}

	c := &Controller{
		clientSet:     clientSet,
		eventRecorder: eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: controllerName}),
		queue:         workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), controllerName),
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

	return c, nil
}

// Run begins watching and syncing.
func (c *Controller) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()
	c.log.Info(fmt.Sprintf("start controller(%s)", c.controllerName))
	defer c.log.Info(fmt.Sprintf("shutting down controller(%s)", c.controllerName))

	c.informer.Informer().HasSynced()

	if !cache.WaitForNamedCacheSync(c.controllerName, stopCh, c.informer.Informer().HasSynced) {
		return
	}
	for i := 0; i < workers; i++ {
		go wait.Until(c.worker, time.Second, stopCh)
	}

	<-stopCh
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
	if err == nil || errors.HasStatusCause(err, v1.NamespaceTerminatingCause) {
		c.queue.Forget(key)
		return
	}

	ns, name, keyErr := cache.SplitMetaNamespaceKey(key.(string))
	if keyErr != nil {
		c.log.Error(err, "Failed to split meta namespace cache key", "cacheKey", key)
	}

	if c.queue.NumRequeues(key) < maxRetries {
		c.log.Info("Error syncing resource", c.controllerName, klog.KRef(ns, name), "err", err)
		c.queue.AddRateLimited(key)
		return
	}

	utilruntime.HandleError(err)
	c.log.Info("Dropping deployment out of the queue", c.controllerName, klog.KRef(ns, name), "err", err)
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
	c.log.Info("Started syncing resource", c.controllerName, klog.KRef(namespace, name), "startTime", startTime)
	defer func() {
		klog.V(4).InfoS("Finished syncing resource", c.controllerName, klog.KRef(namespace, name), "duration", time.Since(startTime))
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
	//unstructuredResource.SetGroupVersionKind(c.resourceRef)

	ctx := context.Background()
	if !match.IsTargetResource(ctx, c.resourceRef, unstructuredResource) {
		return nil
	}

	// get target resource info
	err = c.getTargetResourceInfo(ctx, unstructuredResource)
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

func (c *Controller) getTargetResourceInfo(ctx context.Context, object client.Object) error {
	// find rule
	ruleList, err := resource_aggregate_rule.GetAggregateRuleListWithLabelSelector(ctx, proxy_cfg.ProxyConfig.ProxyClient, c.resourceRef, metav1.NamespaceAll)
	if err != nil {
		return err
	}

	for _, rule := range ruleList.Items {
		policyList, err := resource_aggregate_policy.GetAggregatePolicyListWithLabelSelector(ctx, proxy_cfg.ProxyConfig.ProxyClient, managerCommon.ManagerNamespace, common.NamespacedName{
			Namespace: rule.GetNamespace(),
			Name:      rule.GetName(),
		})
		if err != nil {
			return err
		}
		resourceData, err := cueRender.RenderCue(object, rule.Spec.Rule.Cue, "")
		if err != nil {
			return err
		}
		for _, policy := range policyList.Items {
			resourceDataModel := NewAggregateResourceDataRequest(&rule, &policy, object, resourceData)
			data, err := json.Marshal(resourceDataModel)
			if err != nil {
				return err
			}
			request, err := proxysend.NewAggregateRequest(proxy_cfg.ProxyConfig.Cfg.ClusterName, string(data))
			if err != nil {
				return err
			}
			err = proxysend.SendSyncResourceRequest(request)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func NewAggregateResourceDataRequest(rule *v1alpha1.MultiClusterResourceAggregateRule, policy *v1alpha1.ResourceAggregatePolicy, object client.Object, resourceData []byte) *model.AggregateResourceDataModel {
	data := &model.AggregateResourceDataModel{}
	data.ResourceAggregatePolicy = &common.NamespacedName{
		Namespace: managerCommon.ClusterNamespace(proxy_cfg.ProxyConfig.Cfg.ClusterName),
		Name:      policy.GetName(),
	}
	data.MultiClusterResourceAggregateRule = &common.NamespacedName{
		Namespace: rule.GetNamespace(),
		Name:      rule.GetName(),
	}
	data.TargetResourceData = append(data.TargetResourceData, model.TargetResourceDataModel{
		Namespace: object.GetNamespace(),
		ResourceInfoList: []model.ResourceDataModel{
			model.ResourceDataModel{
				Name:         object.GetName(),
				ResourceData: resourceData,
			},
		},
	})
	return data
}
