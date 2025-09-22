/*
Copyright 2022 The Kube Bind Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package claimedresources

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	dynamicclient "k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamiclister"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	"github.com/kube-bind/kube-bind/pkg/indexers"
	"github.com/kube-bind/kube-bind/pkg/konnector/controllers/cluster/serviceexport/multinsinformer"
	"github.com/kube-bind/kube-bind/pkg/konnector/controllers/dynamic"
	kubebindv1alpha2 "github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2"
	bindclientset "github.com/kube-bind/kube-bind/sdk/client/clientset/versioned"
	bindlisters "github.com/kube-bind/kube-bind/sdk/client/listers/kubebind/v1alpha2"
)

const (
	controllerName = "kube-bind-konnector-claimed-object"
)

// NewController returns a new controller reconciling downstream objects to upstream.
func NewController(
	gvr schema.GroupVersionResource,
	claim kubebindv1alpha2.PermissionClaim,
	apiServiceExport *kubebindv1alpha2.APIServiceExport,
	providerNamespace string,
	consumerConfig, providerConfig *rest.Config,
	consumerDynamicInformer informers.GenericInformer,
	providerDynamicInformer multinsinformer.GetterInformer,
	serviceNamespaceInformer dynamic.Informer[bindlisters.APIServiceNamespaceLister],
) (*controller, error) {
	queue := workqueue.NewTypedRateLimitingQueueWithConfig(workqueue.DefaultTypedControllerRateLimiter[string](), workqueue.TypedRateLimitingQueueConfig[string]{Name: controllerName})

	logger := klog.Background().WithValues("controller", controllerName, "gvr", gvr)

	providerConfig = rest.CopyConfig(providerConfig)
	providerConfig = rest.AddUserAgent(providerConfig, controllerName)

	providerClient, err := dynamicclient.NewForConfig(providerConfig)
	if err != nil {
		return nil, err
	}
	consumerClient, err := dynamicclient.NewForConfig(consumerConfig)
	if err != nil {
		return nil, err
	}

	bindClient, err := bindclientset.NewForConfig(providerConfig)
	if err != nil {
		return nil, err
	}

	dynamicConsumerLister := dynamiclister.New(consumerDynamicInformer.Informer().GetIndexer(), gvr)
	c := &controller{
		queue: queue,

		claim:            claim,
		apiServiceExport: apiServiceExport, // used to establish owner references when create happens on the provider side. TODO: Do we really need this?

		consumerClient:     consumerClient,
		providerClient:     providerClient,
		providerBindClient: bindClient,

		consumerDynamicLister:  dynamicConsumerLister,
		consumerDynamicIndexer: consumerDynamicInformer.Informer().GetIndexer(),

		providerDynamicInformer: providerDynamicInformer,

		serviceNamespaceInformer: serviceNamespaceInformer,

		providerNamespace: providerNamespace,

		readReconciler: readReconciler{
			getServiceNamespace: func(upstreamNamespace string) (*kubebindv1alpha2.APIServiceNamespace, error) {
				sns, err := serviceNamespaceInformer.Informer().GetIndexer().ByIndex(indexers.ServiceNamespaceByNamespace, upstreamNamespace)
				if err != nil {
					return nil, err
				}
				if len(sns) == 0 {
					return nil, errors.NewNotFound(kubebindv1alpha2.SchemeGroupVersion.WithResource("APIServiceNamespace").GroupResource(), upstreamNamespace)
				}
				return sns[0].(*kubebindv1alpha2.APIServiceNamespace), nil
			},
			getConsumerObject: func(ctx context.Context, ns, name string) (*unstructured.Unstructured, error) {
				return consumerClient.Resource(gvr).Namespace(ns).Get(ctx, name, metav1.GetOptions{})
			},
			getProviderObject: func(ns, name string) (*unstructured.Unstructured, error) {
				obj, err := providerDynamicInformer.Get(ns, name)
				if err != nil {
					return nil, err
				}
				return obj.(*unstructured.Unstructured), nil
			},
			createProviderObject: func(ctx context.Context, obj *unstructured.Unstructured) error {
				_, err := providerClient.Resource(gvr).Namespace(obj.GetNamespace()).Create(ctx, obj, metav1.CreateOptions{})
				return err
			},
			updateProviderObject: func(ctx context.Context, obj *unstructured.Unstructured) error {
				_, err := providerClient.Resource(gvr).Namespace(obj.GetNamespace()).Update(ctx, obj, metav1.UpdateOptions{})
				return err
			},
			deleteProviderObject: func(ctx context.Context, ns, name string) error {
				return providerClient.Resource(gvr).Namespace(ns).Delete(ctx, name, metav1.DeleteOptions{})
			},
			deleteConsumerObject: func(ctx context.Context, ns, name string) error {
				return consumerClient.Resource(gvr).Namespace(ns).Delete(ctx, name, metav1.DeleteOptions{})
			},
			updateConsumerObject: func(ctx context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
				return consumerClient.Resource(gvr).Namespace(obj.GetNamespace()).Update(ctx, obj, metav1.UpdateOptions{})
			},
			createConsumerObject: func(ctx context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
				return consumerClient.Resource(gvr).Namespace(obj.GetNamespace()).Create(ctx, obj, metav1.CreateOptions{})
			},
		},
	}

	if _, err = consumerDynamicInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			c.enqueueConsumer(logger, obj)
		},
		UpdateFunc: func(_, newObj interface{}) {
			c.enqueueConsumer(logger, newObj)
		},
		DeleteFunc: func(obj interface{}) {
			c.enqueueConsumer(logger, obj)
		},
	}); err != nil {
		return nil, err
	}

	if err := providerDynamicInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			c.enqueueProvider(logger, obj)
		},
		UpdateFunc: func(_, newObj interface{}) {
			c.enqueueProvider(logger, newObj)
		},
		DeleteFunc: func(obj interface{}) {
			c.enqueueProvider(logger, obj)
		},
	}); err != nil {
		return nil, err
	}

	return c, nil
}

// controller reconciles upstream objects to downstream.
type controller struct {
	queue workqueue.TypedRateLimitingInterface[string]

	claim            kubebindv1alpha2.PermissionClaim
	apiServiceExport *kubebindv1alpha2.APIServiceExport // used to establish owner references when create happens from the consumer side.

	consumerClient     dynamicclient.Interface
	providerClient     dynamicclient.Interface
	providerBindClient bindclientset.Interface

	consumerDynamicLister  dynamiclister.Lister
	consumerDynamicIndexer cache.Indexer

	providerDynamicInformer multinsinformer.GetterInformer

	serviceNamespaceInformer dynamic.Informer[bindlisters.APIServiceNamespaceLister]

	providerNamespace string

	readReconciler
}

func (c *controller) isClaimed(obj *unstructured.Unstructured) bool {
	if c.claim.Selector.All {
		return true
	}

	// Check if obj is selected by label selector
	if c.claim.Selector.LabelSelector != nil {
		selector, err := metav1.LabelSelectorAsSelector(c.claim.Selector.LabelSelector)
		if err != nil {
			return false
		}
		l := obj.GetLabels()
		if l == nil {
			l = make(map[string]string)
		}

		return selector.Matches(labels.Set(l))
	}

	return false
}

func (c *controller) enqueueConsumer(logger klog.Logger, obj interface{}) {
	// handle tombstones
	if tombstone, ok := obj.(cache.DeletedFinalStateUnknown); ok {
		obj = tombstone.Obj
	}
	o, ok := obj.(*unstructured.Unstructured)
	if !ok {
		runtime.HandleError(fmt.Errorf("unexpected type %T in enqueueConsumer", obj))
		return
	}
	if !c.isClaimed(o) {
		return
	}
	logger.V(2).Info("queueing consumer object", "gvr", o.GroupVersionKind().String(), "key", fmt.Sprintf("%s/%s", o.GetNamespace(), o.GetName()))

	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		runtime.HandleError(err)
		return
	}
	ns, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(err)
		return
	}

	if ns != "" { // Namespaced object.
		sn, err := c.serviceNamespaceInformer.Lister().APIServiceNamespaces(c.providerNamespace).Get(ns)
		if err != nil {
			if errors.IsNotFound(err) {
				// No namespace - create one.
				// TODO: This is not quite right place for this code. We should refactor this to be somewhere else.
				_, err := c.createServiceNamespace(context.TODO(), &kubebindv1alpha2.APIServiceNamespace{
					ObjectMeta: metav1.ObjectMeta{
						Name:      ns,
						Namespace: c.providerNamespace,
						OwnerReferences: []metav1.OwnerReference{
							*metav1.NewControllerRef(c.apiServiceExport, kubebindv1alpha2.SchemeGroupVersion.WithKind("APIServiceExport")),
						},
					},
				})
				if err != nil && !errors.IsAlreadyExists(err) {
					runtime.HandleError(fmt.Errorf("failed to create APIServiceNamespace %q: %w", ns, err))
					return
				}
				// Requeue when the APIServiceNamespace is created.
				logger.V(2).Info("created APIServiceNamespace, requeueing", "namespace", ns)
				// Once APIServiceNamespace is created, the informer will pick it up and requeue the objects.
				return
			}
		}
		if sn.Status.Namespace == "" {
			return // not ready yet
		}
		if sn.Namespace == c.providerNamespace && sn.Status.Namespace != "" {
			key := fmt.Sprintf("%s/%s", sn.Status.Namespace, name)
			logger.V(2).Info("queueing Unstructured", "key", key, "reason", "Consumer")
			c.queue.Add(key)
			return
		}
		return
	}

	logger.V(2).Info("queueing Unstructured", "key", key)
	c.queue.Add(key)
}

func (c *controller) enqueueProvider(logger klog.Logger, obj interface{}) {
	upstreamKey, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		runtime.HandleError(err)
		return
	}
	ns, _, err := cache.SplitMetaNamespaceKey(upstreamKey)
	if err != nil {
		runtime.HandleError(err)
		return
	}

	if ns != "" {
		sns, err := c.serviceNamespaceInformer.Informer().GetIndexer().ByIndex(indexers.ServiceNamespaceByNamespace, ns)
		if err != nil {
			if !errors.IsNotFound(err) {
				runtime.HandleError(err)
			}
			return
		}
		for _, obj := range sns {
			sn := obj.(*kubebindv1alpha2.APIServiceNamespace)
			if sn.Namespace == c.providerNamespace {
				logger.V(2).Info("queueing Unstructured", "key", upstreamKey)
				c.queue.Add(upstreamKey)
				return
			}
		}
		return
	}

	logger.V(2).Info("queueing Unstructured", "key", upstreamKey)
	c.queue.Add(upstreamKey)
}

func (c *controller) enqueueServiceNamespace(logger klog.Logger, obj interface{}) {
	snKey, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		runtime.HandleError(err)
		return
	}
	ns, name, err := cache.SplitMetaNamespaceKey(snKey)
	if err != nil {
		runtime.HandleError(err)
		return
	}
	if ns != c.providerNamespace {
		return // not for us
	}

	sn, err := c.serviceNamespaceInformer.Lister().APIServiceNamespaces(ns).Get(name)
	if err != nil {
		runtime.HandleError(err)
		return
	}

	if sn.Status.Namespace == "" {
		return // not ready
	}

	logger.Info("enqueueing service namespace", "upstreamNamespace", sn.Status.Namespace)
	objs, err := c.providerDynamicInformer.List(sn.Status.Namespace)
	if err != nil {
		runtime.HandleError(err)
		return
	}
	for _, obj := range objs {
		logger.Info("enqueueing provider object", "obj", obj)

		key, err := cache.MetaNamespaceKeyFunc(obj)
		if err != nil {
			runtime.HandleError(err)
			continue
		}
		logger.V(2).Info("queueing Unstructured", "key", key, "reason", "APIServiceNamespace", "ServiceNamespaceKey", key)
		c.queue.Add(key)
	}

	// We need to list all the object which might not got synced at consumer side too:
	selector, err := metav1.LabelSelectorAsSelector(c.claim.Selector.LabelSelector)
	if err != nil {
		return // cannot happen, we validated this earlier
	}
	objects, err := c.consumerDynamicLister.List(selector)
	if err != nil {
		runtime.HandleError(err)
		return
	}
	for _, obj := range objects {
		logger.Info("enqueueing consumer object", "obj", obj)

		key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
		if err != nil {
			runtime.HandleError(err)
			return
		}
		_, name, err := cache.SplitMetaNamespaceKey(key)
		if err != nil {
			runtime.HandleError(err)
			return
		}
		key = fmt.Sprintf("%s/%s", sn.Status.Namespace, name)

		logger.V(2).Info("queueing Unstructured", "key", key, "reason", "APIServiceNamespace", "ConsumerObject", key)
		c.queue.Add(key)
	}
}

// Start starts the controller, which stops when ctx.Done() is closed.
func (c *controller) Start(ctx context.Context, numThreads int) {
	defer runtime.HandleCrash()
	defer c.queue.ShutDown()

	logger := klog.FromContext(ctx).WithValues("controller", controllerName)

	logger.Info("Starting controller")
	defer logger.Info("Shutting down controller")

	c.serviceNamespaceInformer.Informer().AddDynamicEventHandler(ctx, controllerName, cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			c.enqueueServiceNamespace(logger, obj)
		},
		UpdateFunc: func(_, newObj interface{}) {
			c.enqueueServiceNamespace(logger, newObj)
		},
		DeleteFunc: func(obj interface{}) {
			c.enqueueServiceNamespace(logger, obj)
		},
	})

	for i := 0; i < numThreads; i++ {
		go wait.UntilWithContext(ctx, c.startWorker, time.Second)
	}

	<-ctx.Done()
}

func (c *controller) startWorker(ctx context.Context) {
	defer runtime.HandleCrash()

	for c.processNextWorkItem(ctx) {
	}
}

func (c *controller) processNextWorkItem(ctx context.Context) bool {
	// Wait until there is a new item in the working queue
	key, quit := c.queue.Get()
	if quit {
		return false
	}

	logger := klog.FromContext(ctx).WithValues("key", key)
	ctx = klog.NewContext(ctx, logger)
	logger.V(2).Info("processing key")

	// No matter what, tell the queue we're done with this key, to unblock
	// other workers.
	defer c.queue.Done(key)

	if err := c.process(ctx, key); err != nil {
		runtime.HandleError(fmt.Errorf("%q controller failed to sync %q, err: %w", controllerName, key, err))
		c.queue.AddRateLimited(key)
		return true
	}
	c.queue.Forget(key)
	return true
}

func (c *controller) process(ctx context.Context, key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(err)
		return nil // we cannot do anything
	}

	return c.reconcile(ctx, namespace, name)
}

func (c *controller) createServiceNamespace(ctx context.Context, sns *kubebindv1alpha2.APIServiceNamespace) (*kubebindv1alpha2.APIServiceNamespace, error) {
	return c.providerBindClient.KubeBindV1alpha2().APIServiceNamespaces(sns.Namespace).Create(ctx, sns, metav1.CreateOptions{})
}

// configMap test-namespace/test
// goes into index - indexers.ServiceNamespaceByNamespace with "local name" ->

// providerNamespace/<api-service-namspaces.....>
