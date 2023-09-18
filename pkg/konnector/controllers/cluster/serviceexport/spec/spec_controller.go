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

package spec

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	dynamicclient "k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamiclister"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	"k8s.io/utils/pointer"

	kubebindv1alpha1 "github.com/kube-bind/kube-bind/pkg/apis/kubebind/v1alpha1"
	bindclient "github.com/kube-bind/kube-bind/pkg/client/clientset/versioned"
	bindlisters "github.com/kube-bind/kube-bind/pkg/client/listers/kubebind/v1alpha1"
	"github.com/kube-bind/kube-bind/pkg/indexers"
	clusterscoped "github.com/kube-bind/kube-bind/pkg/konnector/controllers/cluster/serviceexport/cluster-scoped"
	"github.com/kube-bind/kube-bind/pkg/konnector/controllers/cluster/serviceexport/multinsinformer"
	"github.com/kube-bind/kube-bind/pkg/konnector/controllers/dynamic"
)

const (
	controllerName = "kube-bind-konnector-cluster-spec"

	applyManager = "kube-bind.io"
)

// NewController returns a new controller reconciling downstream objects to upstream.
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

	providerConfig = rest.CopyConfig(providerConfig)
	providerConfig = rest.AddUserAgent(providerConfig, controllerName)

	providerClient, err := dynamicclient.NewForConfig(providerConfig)
	if err != nil {
		return nil, err
	}
	providerBindClient, err := bindclient.NewForConfig(providerConfig)
	if err != nil {
		return nil, err
	}
	consumerClient, err := dynamicclient.NewForConfig(consumerConfig)
	if err != nil {
		return nil, err
	}

	dynamicConsumerLister := dynamiclister.New(consumerDynamicInformer.Informer().GetIndexer(), gvr)
	c := &controller{
		queue: queue,

		consumerClient: consumerClient,
		providerClient: providerClient,

		consumerDynamicLister:  dynamicConsumerLister,
		consumerDynamicIndexer: consumerDynamicInformer.Informer().GetIndexer(),

		providerDynamicInformer: providerDynamicInformer,

		serviceNamespaceInformer: serviceNamespaceInformer,

		reconciler: reconciler{
			providerNamespace: providerNamespace,
			getServiceNamespace: func(name string) (*kubebindv1alpha1.APIServiceNamespace, error) {
				return serviceNamespaceInformer.Lister().APIServiceNamespaces(providerNamespace).Get(name)
			},
			createServiceNamespace: func(ctx context.Context, sn *kubebindv1alpha1.APIServiceNamespace) (*kubebindv1alpha1.APIServiceNamespace, error) {
				return providerBindClient.KubeBindV1alpha1().APIServiceNamespaces(providerNamespace).Create(ctx, sn, metav1.CreateOptions{})
			},
			getProviderObject: func(ns, name string) (*unstructured.Unstructured, error) {
				if ns != "" {
					obj, err := providerDynamicInformer.Get(ns, name)
					if err != nil {
						return nil, err
					}
					return obj.(*unstructured.Unstructured), nil
				}
				got, err := providerDynamicInformer.Get(ns, clusterscoped.Prepend(name, providerNamespace))
				if err != nil {
					return nil, err
				}
				obj := got.(*unstructured.Unstructured).DeepCopy()
				err = clusterscoped.TranslateFromUpstream(obj)
				if err != nil {
					return nil, err
				}
				return obj, nil
			},
			createProviderObject: func(ctx context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
				if ns := obj.GetNamespace(); ns != "" {
					return providerClient.Resource(gvr).Namespace(obj.GetNamespace()).Create(ctx, obj, metav1.CreateOptions{})
				}
				err := clusterscoped.TranslateFromDownstream(obj, providerNamespace, providerNamespaceUID)
				if err != nil {
					return nil, err
				}
				created, err := providerClient.Resource(gvr).Namespace(obj.GetNamespace()).Create(ctx, obj, metav1.CreateOptions{})
				if err != nil {
					return nil, err
				}
				err = clusterscoped.TranslateFromUpstream(created)
				if err != nil {
					return nil, err
				}
				return created, nil
			},
			updateProviderObject: func(ctx context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
				ns := obj.GetNamespace()
				if ns == "" {
					if err := clusterscoped.TranslateFromDownstream(obj, providerNamespace, providerNamespaceUID); err != nil {
						return nil, err
					}
				}
				data, err := json.Marshal(obj.Object)
				if err != nil {
					return nil, err
				}
				patched, err := providerClient.Resource(gvr).Namespace(obj.GetNamespace()).Patch(ctx,
					obj.GetName(), types.ApplyPatchType, data, metav1.PatchOptions{FieldManager: applyManager, Force: pointer.Bool(true)},
				)
				if err != nil {
					return nil, err
				}
				if ns == "" {
					err = clusterscoped.TranslateFromUpstream(patched)
					if err != nil {
						return nil, err
					}
					return patched, nil
				}
				return patched, nil
			},
			deleteProviderObject: func(ctx context.Context, ns, name string) error {
				if ns == "" {
					name = clusterscoped.Prepend(name, providerNamespace)
				}
				return providerClient.Resource(gvr).Namespace(ns).Delete(ctx, name, metav1.DeleteOptions{})
			},
			updateConsumerObject: func(ctx context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
				return consumerClient.Resource(gvr).Namespace(obj.GetNamespace()).Update(ctx, obj, metav1.UpdateOptions{})
			},
			requeue: func(obj *unstructured.Unstructured, after time.Duration) error {
				key, err := cache.MetaNamespaceKeyFunc(obj)
				if err != nil {
					return err
				}
				queue.AddAfter(key, after)
				return nil
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

// controller reconciles downstream objects to upstream.
type controller struct {
	queue workqueue.RateLimitingInterface

	consumerClient dynamicclient.Interface
	providerClient dynamicclient.Interface

	consumerDynamicLister  dynamiclister.Lister
	consumerDynamicIndexer cache.Indexer

	providerDynamicInformer multinsinformer.GetterInformer

	serviceNamespaceInformer dynamic.Informer[bindlisters.APIServiceNamespaceLister]

	reconciler
}

func (c *controller) enqueueConsumer(logger klog.Logger, obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		runtime.HandleError(err)
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
	ns, name, err := cache.SplitMetaNamespaceKey(upstreamKey)
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
			sn := obj.(*kubebindv1alpha1.APIServiceNamespace)
			if sn.Namespace == c.providerNamespace {
				key := fmt.Sprintf("%s/%s", sn.Name, name)
				logger.V(2).Info("queueing Unstructured", "key", key)
				c.queue.Add(key)
				return
			}
		}
		return
	}

	if clusterscoped.Behead(upstreamKey, c.providerNamespace) == upstreamKey {
		logger.V(3).Info("skipping because consumer mismatch", "upstreamKey", upstreamKey)
		return
	}
	downstreamKey := clusterscoped.Behead(upstreamKey, c.providerNamespace)
	logger.V(2).Info("queueing Unstructured", "key", downstreamKey)
	c.queue.Add(downstreamKey)
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

	objs, err := c.consumerDynamicIndexer.ByIndex(cache.NamespaceIndex, name)
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

	var obj *unstructured.Unstructured
	if ns == "" {
		obj, err = c.consumerDynamicLister.Get(name)
	} else {
		obj, err = c.consumerDynamicLister.Namespace(ns).Get(name)
	}
	if err != nil && !errors.IsNotFound(err) {
		return err
	} else if errors.IsNotFound(err) {
		logger.V(2).Info("Downstream object disappeared")
		return nil
	}

	return c.reconcile(ctx, obj)
}
