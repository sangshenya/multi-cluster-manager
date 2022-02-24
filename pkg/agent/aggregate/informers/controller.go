package informers

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/runtime"

	"harmonycloud.cn/stellaris/pkg/agent/aggregate/config"
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
)

type controller struct {
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

func NewController(controllerName string, resourceRef *metav1.GroupVersionKind, clientSet clientset.Interface, informer informers.GenericInformer) (*controller, error) {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartStructuredLogging(0)
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: clientSet.CoreV1().Events("")})

	if clientSet != nil && clientSet.CoreV1().RESTClient().GetRateLimiter() != nil {
		if err := ratelimiter.RegisterMetricAndTrackRateLimiterUsage(controllerName, clientSet.CoreV1().RESTClient().GetRateLimiter()); err != nil {
			return nil, err
		}
	}

	c := &controller{
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
func (c *controller) Run(workers int, stopCh <-chan struct{}) {
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
func (c *controller) worker() {
	for c.processNextWorkItem() {
	}
}

func (c *controller) processNextWorkItem() bool {
	key, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(key)

	err := c.syncHandler(key.(string))
	c.handleErr(err, key)

	return true
}

func (c *controller) handleErr(err error, key interface{}) {
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
func (c *controller) enqueue(resource client.Object) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(resource)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", resource, err))
		return
	}

	c.queue.Add(key)
}

// sync func
func (c *controller) syncResource(key string) error {
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
	if !config.IsTargetResource(ctx, c.resourceRef, unstructuredResource) {
		return nil
	}

	//

}

// event handler
func (c *controller) addResource(obj interface{}) {

}

func (c *controller) updateResource(old, cur interface{}) {

}

func (c *controller) deleteResource(obj interface{}) {

}
