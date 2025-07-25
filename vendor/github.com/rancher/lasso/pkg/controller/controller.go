/*
Package controller contains interfaces, structs, and functions that can be used to create shared controllers for
managing clients, caches, and handlers for multiple types aka GroupVersionKinds.
*/
package controller

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/rancher/lasso/pkg/log"
	"github.com/rancher/lasso/pkg/metrics"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

const maxTimeout2min = 2 * time.Minute

type Handler interface {
	OnChange(key string, obj runtime.Object) error
}

type ResourceVersionGetter interface {
	GetResourceVersion() string
}

type HandlerFunc func(key string, obj runtime.Object) error

func (h HandlerFunc) OnChange(key string, obj runtime.Object) error {
	return h(key, obj)
}

type retryAfterError struct {
	error
	duration time.Duration
}

type Controller interface {
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, delay time.Duration)
	EnqueueKey(key string)
	Informer() cache.SharedIndexInformer
	Start(ctx context.Context, workers int) error
}

type controller struct {
	startLock sync.Mutex

	name        string
	ctxID       string
	workqueue   workqueue.RateLimitingInterface
	rateLimiter workqueue.RateLimiter
	informer    cache.SharedIndexInformer
	handler     Handler
	gvk         schema.GroupVersionKind
	startKeys   []startKey
	started     bool
	startCache  func(context.Context) error
}

type startKey struct {
	key   string
	after time.Duration
}

type Options struct {
	RateLimiter            workqueue.RateLimiter
	SyncOnlyChangedObjects bool
}

func New(name string, informer cache.SharedIndexInformer, startCache func(context.Context) error, handler Handler, opts *Options) Controller {
	opts = applyDefaultOptions(opts)

	controller := &controller{
		name:        name,
		handler:     handler,
		informer:    informer,
		rateLimiter: opts.RateLimiter,
		startCache:  startCache,
	}

	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.handleObject,
		UpdateFunc: func(old, new interface{}) {
			if !opts.SyncOnlyChangedObjects || old.(ResourceVersionGetter).GetResourceVersion() != new.(ResourceVersionGetter).GetResourceVersion() {
				// If syncOnlyChangedObjects is disabled, objects will be handled regardless of whether an update actually took place.
				// Otherwise, objects will only be handled if they have changed
				controller.handleObject(new)
			}
		},
		DeleteFunc: controller.handleObject,
	})

	return controller
}

func applyDefaultOptions(opts *Options) *Options {
	var newOpts Options
	if opts != nil {
		newOpts = *opts
	}

	// from failure 0 to 12: exponential growth in delays (5 ms * 2 ^ failures)
	// from failure 13 to 30: 30s delay
	// from failure 31 on: 120s delay (2 minutes)
	if newOpts.RateLimiter == nil {
		newOpts.RateLimiter = workqueue.NewMaxOfRateLimiter(
			workqueue.NewItemFastSlowRateLimiter(time.Millisecond, maxTimeout2min, 30),
			workqueue.NewItemExponentialFailureRateLimiter(5*time.Millisecond, 30*time.Second),
		)
	}
	return &newOpts
}

func (c *controller) Informer() cache.SharedIndexInformer {
	return c.informer
}

func (c *controller) GroupVersionKind() schema.GroupVersionKind {
	return c.gvk
}

func (c *controller) run(workers int, stopCh <-chan struct{}) {
	c.startLock.Lock()
	// we have to defer queue creation until we have a stopCh available because a workqueue
	// will create a goroutine under the hood.  It we instantiate a workqueue we must have
	// a mechanism to Shutdown it down.  Without the stopCh we don't know when to shutdown
	// the queue and release the goroutine
	c.workqueue = workqueue.NewNamedRateLimitingQueue(c.rateLimiter, c.name)
	for _, start := range c.startKeys {
		if start.after == 0 {
			c.workqueue.Add(start.key)
		} else {
			c.workqueue.AddAfter(start.key, start.after)
		}
	}
	c.startKeys = nil
	c.startLock.Unlock()

	defer utilruntime.HandleCrash()
	defer func() {
		c.workqueue.ShutDown()
	}()

	// Start the informer factories to begin populating the informer caches
	log.Infof("Starting %s controller", c.name)

	for i := 0; i < workers; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	<-stopCh
	c.startLock.Lock()
	defer c.startLock.Unlock()
	c.started = false
	log.Infof("Shutting down %s workers", c.name)
}

func (c *controller) Start(ctx context.Context, workers int) error {
	c.startLock.Lock()
	defer c.startLock.Unlock()

	if c.started {
		return nil
	}

	if err := c.startCache(ctx); err != nil {
		return err
	}

	if ok := cache.WaitForCacheSync(ctx.Done(), c.informer.HasSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	c.ctxID = metrics.ContextID(ctx)
	go c.run(workers, ctx.Done())
	c.started = true
	return nil
}

func (c *controller) runWorker() {
	for c.processNextWorkItem() {
	}
}

func (c *controller) processNextWorkItem() bool {
	obj, shutdown := c.workqueue.Get()

	if shutdown {
		return false
	}

	if err := c.processSingleItem(obj); err != nil {
		if !strings.Contains(err.Error(), "please apply your changes to the latest version and try again") {
			log.Errorf("%v", err)
		}
		return true
	}

	return true
}

func (c *controller) processSingleItem(obj interface{}) error {
	var (
		key string
		ok  bool
	)

	defer c.workqueue.Done(obj)

	if key, ok = obj.(string); !ok {
		c.workqueue.Forget(obj)
		log.Errorf("expected string in workqueue but got %#v", obj)
		return nil
	}
	if err := c.syncHandler(key); err != nil {
		var retryAfter *retryAfterError
		if errors.As(err, &retryAfter) {
			c.workqueue.AddAfter(key, retryAfter.duration)
			return retryAfter.error
		}
		c.workqueue.AddRateLimited(key)
		return fmt.Errorf("error syncing '%s': %s, requeuing", key, err.Error())
	}

	c.workqueue.Forget(obj)
	return nil
}

func (c *controller) syncHandler(key string) error {
	obj, exists, err := c.informer.GetStore().GetByKey(key)
	if err != nil {
		metrics.IncTotalHandlerExecutions(c.ctxID, c.name, "", true)
		return err
	}
	if !exists {
		return c.handler.OnChange(key, nil)
	}

	return c.handler.OnChange(key, obj.(runtime.Object))
}

func (c *controller) EnqueueKey(key string) {
	c.startLock.Lock()
	defer c.startLock.Unlock()

	if c.workqueue == nil {
		c.startKeys = append(c.startKeys, startKey{key: key})
	} else {
		c.workqueue.Add(key)
	}
}

func (c *controller) Enqueue(namespace, name string) {
	key := keyFunc(namespace, name)

	c.startLock.Lock()
	defer c.startLock.Unlock()

	if c.workqueue == nil {
		c.startKeys = append(c.startKeys, startKey{key: key})
	} else {
		c.workqueue.AddRateLimited(key)
	}
}

func (c *controller) EnqueueAfter(namespace, name string, duration time.Duration) {
	key := keyFunc(namespace, name)

	c.startLock.Lock()
	defer c.startLock.Unlock()

	if c.workqueue == nil {
		c.startKeys = append(c.startKeys, startKey{key: key, after: duration})
	} else {
		c.workqueue.AddAfter(key, duration)
	}
}

func keyFunc(namespace, name string) string {
	if namespace == "" {
		return name
	}
	return namespace + "/" + name
}

func (c *controller) enqueue(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		log.Errorf("%v", err)
		return
	}
	c.startLock.Lock()
	if c.workqueue == nil {
		c.startKeys = append(c.startKeys, startKey{key: key})
	} else {
		c.workqueue.Add(key)
	}
	c.startLock.Unlock()
}

func (c *controller) handleObject(obj interface{}) {
	if _, ok := obj.(metav1.Object); !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			log.Errorf("error decoding object, invalid type")
			return
		}
		newObj, ok := tombstone.Obj.(metav1.Object)
		if !ok {
			log.Errorf("error decoding object tombstone, invalid type")
			return
		}
		obj = newObj
	}
	c.enqueue(obj)
}
