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

package status

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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

	kubebindv1alpha1 "github.com/kube-bind/kube-bind/pkg/apis/kubebind/v1alpha1"
	bindlisters "github.com/kube-bind/kube-bind/pkg/client/listers/kubebind/v1alpha1"
	"github.com/kube-bind/kube-bind/pkg/indexers"
	clusterscoped "github.com/kube-bind/kube-bind/pkg/konnector/controllers/cluster/serviceexport/cluster-scoped"
	"github.com/kube-bind/kube-bind/pkg/konnector/controllers/cluster/serviceexport/multinsinformer"
	"github.com/kube-bind/kube-bind/pkg/konnector/controllers/dynamic"
)

const (
	controllerName = "kube-bind-konnector-cluster-status"
)

// NewController returns a new controller reconciling status of upstream to downstream.
func NewController(
	gvr schema.GroupVersionResource,
	providerNamespace string,
	providerNamespaceUID string,
	consumerConfig, providerConfig *rest.Config,
	consumerDynamicInformer informers.GenericInformer,
	providerDynamicInformer multinsinformer.GetterInformer,
	serviceNamespaceInformer dynamic.Informer[bindlisters.APIServiceNamespaceLister],
) (*controller, error) {
	queue := workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), controllerName)

	logger := klog.Background().WithValues("controller", controllerName)

	consumerConfig = rest.CopyConfig(consumerConfig)
	consumerConfig = rest.AddUserAgent(consumerConfig, controllerName)

	providerConfig = rest.CopyConfig(providerConfig)
	providerConfig = rest.AddUserAgent(providerConfig, controllerName)

	consumerClient, err := dynamicclient.NewForConfig(consumerConfig)
	if err != nil {
		return nil, err
	}
	providerClient, err := dynamicclient.NewForConfig(providerConfig)
	if err != nil {
		return nil, err
	}

	dynamicConsumerLister := dynamiclister.New(consumerDynamicInformer.Informer().GetIndexer(), gvr)
	c := &controller{
		queue: queue,

		gvr:               gvr,
		providerNamespace: providerNamespace,

		consumerClient: consumerClient,
		providerClient: providerClient,

		consumerDynamicLister:  dynamicConsumerLister,
		consumerDynamicIndexer: consumerDynamicInformer.Informer().GetIndexer(),

		providerDynamicInformer: providerDynamicInformer,

		serviceNamespaceInformer: serviceNamespaceInformer,

		reconciler: reconciler{
			getServiceNamespace: func(upstreamNamespace string) (*kubebindv1alpha1.APIServiceNamespace, error) {
				sns, err := serviceNamespaceInformer.Informer().GetIndexer().ByIndex(indexers.ServiceNamespaceByNamespace, upstreamNamespace)
				if err != nil {
					return nil, err
				}
				if len(sns) == 0 {
					return nil, errors.NewNotFound(kubebindv1alpha1.SchemeGroupVersion.WithResource("APIServiceNamespace").GroupResource(), upstreamNamespace)
				}
				return sns[0].(*kubebindv1alpha1.APIServiceNamespace), nil
			},
			getConsumerObject: func(ns, name string) (*unstructured.Unstructured, error) {
				if ns != "" {
					return dynamicConsumerLister.Namespace(ns).Get(name)
				}
				got, err := dynamicConsumerLister.Get(clusterscoped.Behead(name, providerNamespace))
				if err != nil {
					return nil, err
				}
				obj := got.DeepCopy()
				err = clusterscoped.TranslateFromDownstream(obj, providerNamespace, providerNamespaceUID)
				if err != nil {
					return nil, err
				}
				return obj, nil
			},
			updateConsumerObjectStatus: func(ctx context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
				ns := obj.GetNamespace()
				if ns == "" {
					if err := clusterscoped.TranslateFromUpstream(obj); err != nil {
						return nil, err
					}
				}
				updated, err := consumerClient.Resource(gvr).Namespace(obj.GetNamespace()).UpdateStatus(ctx, obj, metav1.UpdateOptions{})
				if err != nil {
					return nil, err
				}
				if ns == "" {
					err = clusterscoped.TranslateFromDownstream(updated, providerNamespace, providerNamespaceUID)
					if err != nil {
						return nil, err
					}
					return updated, nil
				}
				return updated, nil
			},
			deleteProviderObject: func(ctx context.Context, ns, name string) error {
				return providerClient.Resource(gvr).Namespace(ns).Delete(ctx, name, metav1.DeleteOptions{})
			},
		},
	}

	consumerDynamicInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			c.enqueueConsumer(logger, obj)
		},
		UpdateFunc: func(_, newObj interface{}) {
			c.enqueueConsumer(logger, newObj)
		},
		DeleteFunc: func(obj interface{}) {
			c.enqueueConsumer(logger, obj)
		},
	})

	providerDynamicInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			c.enqueueProvider(logger, obj)
		},
		UpdateFunc: func(_, newObj interface{}) {
			c.enqueueProvider(logger, newObj)
		},
		DeleteFunc: func(obj interface{}) {
			c.enqueueProvider(logger, obj)
		},
	})

	return c, nil
}

// controller reconciles status of upstream to downstream.
type controller struct {
	queue workqueue.RateLimitingInterface

	gvr               schema.GroupVersionResource
	providerNamespace string

	consumerClient, providerClient dynamicclient.Interface

	consumerDynamicLister  dynamiclister.Lister
	consumerDynamicIndexer cache.Indexer

	providerDynamicInformer multinsinformer.GetterInformer

	serviceNamespaceInformer dynamic.Informer[bindlisters.APIServiceNamespaceLister]

	reconciler
}

func (c *controller) enqueueProvider(logger klog.Logger, obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		runtime.HandleError(err)
		return
	}
	ns, _, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(err)
		return
	}
	if ns != "" {
		sns, err := c.serviceNamespaceInformer.Informer().GetIndexer().ByIndex(indexers.ServiceNamespaceByNamespace, ns)
		if err != nil {
			runtime.HandleError(err)
			return
		}
		for _, obj := range sns {
			sns := obj.(*kubebindv1alpha1.APIServiceNamespace)
			if sns.Namespace == c.providerNamespace {
				logger.V(2).Info("queueing Unstructured", "key", key)
				c.queue.Add(key)
				return
			}
		}
		logger.V(3).Info("skipping because consumer mismatch", "key", key)
		return
	}

	if clusterscoped.Behead(key, c.providerNamespace) == key {
		logger.V(3).Info("skipping because consumer mismatch", "key", key)
		return
	}
	logger.V(2).Info("queueing Unstructured", "key", key)
	c.queue.Add(key)
}

func (c *controller) enqueueConsumer(logger klog.Logger, obj interface{}) {
	downstreamKey, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		runtime.HandleError(err)
		return
	}
	ns, name, err := cache.SplitMetaNamespaceKey(downstreamKey)
	if err != nil {
		runtime.HandleError(err)
		return
	}

	if ns != "" {
		sn, err := c.serviceNamespaceInformer.Lister().APIServiceNamespaces(c.providerNamespace).Get(ns)
		if err != nil {
			if !errors.IsNotFound(err) {
				runtime.HandleError(err)
			}
			return
		}
		if sn.Namespace == c.providerNamespace && sn.Status.Namespace != "" {
			key := fmt.Sprintf("%s/%s", sn.Status.Namespace, name)
			logger.V(2).Info("queueing Unstructured", "key", key)
			c.queue.Add(key)
			return
		}
		return
	}

	upstreamKey := clusterscoped.Prepend(downstreamKey, c.providerNamespace)
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
	objs, err := c.providerDynamicInformer.List(sn.Status.Namespace)
	if err != nil {
		runtime.HandleError(err)
		return
	}
	for _, obj := range objs {
		key, err := cache.MetaNamespaceKeyFunc(obj)
		if err != nil {
			runtime.HandleError(err)
			continue
		}
		logger.V(2).Info("queueing Unstructured", "key", key, "reason", "APIServiceNamespace", "ServiceNamespaceKey", key)
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
	k, quit := c.queue.Get()
	if quit {
		return false
	}
	key := k.(string)

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
	ns, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(err)
		return nil // we cannot do anything
	}

	logger := klog.FromContext(ctx)

	obj, err := c.providerDynamicInformer.Get(ns, name)
	if err != nil && !errors.IsNotFound(err) {
		return err
	} else if errors.IsNotFound(err) {
		logger.V(2).Info("Upstream object disappeared")

		downstream, err := c.consumerDynamicLister.Namespace(ns).Get(name)
		if err != nil && !errors.IsNotFound(err) {
			return err
		} else if err == nil {
			if _, err := c.removeDownstreamFinalizer(ctx, downstream); err != nil {
				return err
			}
		}

		return nil
	}

	return c.reconcile(ctx, obj.(*unstructured.Unstructured))
}

func (c *controller) removeDownstreamFinalizer(ctx context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	logger := klog.FromContext(ctx)

	var finalizers []string
	found := false
	for _, f := range obj.GetFinalizers() {
		if f == kubebindv1alpha1.DownstreamFinalizer {
			found = true
			continue
		}
		finalizers = append(finalizers, f)
	}

	if found {
		logger.V(2).Info("removing finalizer from downstream object")
		obj = obj.DeepCopy()
		obj.SetFinalizers(finalizers)
		var err error
		if obj, err = c.consumerClient.Resource(c.gvr).Namespace(obj.GetNamespace()).Update(ctx, obj, metav1.UpdateOptions{}); err != nil && !errors.IsNotFound(err) {
			return nil, err
		}
	}

	return obj, nil
}
