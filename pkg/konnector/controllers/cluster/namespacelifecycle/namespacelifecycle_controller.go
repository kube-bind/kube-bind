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

package namespacelifecycle

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	kubernetesclient "k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	"github.com/kube-bind/kube-bind/pkg/konnector/controllers/dynamic"
	kubebindv1alpha2 "github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2"
	bindclient "github.com/kube-bind/kube-bind/sdk/client/clientset/versioned"
	bindinformers "github.com/kube-bind/kube-bind/sdk/client/informers/externalversions/kubebind/v1alpha2"
	bindlisters "github.com/kube-bind/kube-bind/sdk/client/listers/kubebind/v1alpha2"
)

const (
	controllerName = "kube-bind-konnector-namespacelifecycle"
)

// NewController returns a new controller lifecycle for ServiceNamespaces and Namespaces.
func NewController(
	providerConfig *rest.Config,
	consumerConfig *rest.Config,
	providerNamespace string,
	serviceNamespaceInformer bindinformers.APIServiceNamespaceInformer,
	namespaceInformer dynamic.Informer[corelisters.NamespaceLister],
) (*controller, error) {
	queue := workqueue.NewTypedRateLimitingQueueWithConfig(workqueue.DefaultTypedControllerRateLimiter[string](), workqueue.TypedRateLimitingQueueConfig[string]{Name: controllerName})

	logger := klog.Background().WithValues("controller", controllerName)

	providerConfig = rest.CopyConfig(providerConfig)
	providerConfig = rest.AddUserAgent(providerConfig, controllerName)

	consumerConfig = rest.CopyConfig(consumerConfig)
	consumerConfig = rest.AddUserAgent(consumerConfig, controllerName)

	bindClient, err := bindclient.NewForConfig(providerConfig)
	if err != nil {
		return nil, err
	}
	consumerKubeClient, err := kubernetesclient.NewForConfig(consumerConfig)
	if err != nil {
		return nil, err
	}

	c := &controller{
		queue: queue,

		providerBindClient: bindClient,
		consumerKubeClient: consumerKubeClient,

		serviceNamespaceLister:  serviceNamespaceInformer.Lister(),
		serviceNamespaceIndexer: serviceNamespaceInformer.Informer().GetIndexer(),

		namespaceInformer: namespaceInformer,

		providerNamespace: providerNamespace,

		getNamespace: namespaceInformer.Lister().Get,

		getServiceNamespace: func(ns, name string) (*kubebindv1alpha2.APIServiceNamespace, error) {
			return serviceNamespaceInformer.Lister().APIServiceNamespaces(ns).Get(name)
		},
		deleteServiceNamespace: func(ctx context.Context, ns, name string) error {
			return bindClient.KubeBindV1alpha2().APIServiceNamespaces(ns).Delete(ctx, name, metav1.DeleteOptions{})
		},
	}

	if _, err := serviceNamespaceInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj any) {
			c.enqueueServiceNamespace(logger, obj)
		},
		UpdateFunc: func(_, newObj any) {
			c.enqueueServiceNamespace(logger, newObj)
		},
		DeleteFunc: func(obj any) {
			c.enqueueServiceNamespace(logger, obj)
		},
	}); err != nil {
		return nil, err
	}

	return c, nil
}

// controller reconciles ServiceNamespaces by creating a Namespace for each, and deleting it if
// the APIServiceNamespace is deleted.
type controller struct {
	queue workqueue.TypedRateLimitingInterface[string]

	providerBindClient bindclient.Interface
	consumerKubeClient kubernetesclient.Interface

	namespaceInformer dynamic.Informer[corelisters.NamespaceLister]

	providerNamespace string

	serviceNamespaceLister  bindlisters.APIServiceNamespaceLister
	serviceNamespaceIndexer cache.Indexer

	getNamespace           func(name string) (*corev1.Namespace, error)
	getServiceNamespace    func(ns, name string) (*kubebindv1alpha2.APIServiceNamespace, error)
	deleteServiceNamespace func(ctx context.Context, ns, name string) error
}

func (c *controller) enqueueServiceNamespace(logger klog.Logger, obj any) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		runtime.HandleError(err)
		return
	}

	logger.V(2).Info("queueing APIServiceNamespace", "key", key)
	c.queue.Add(key)
}

func (c *controller) enqueueNamespace(logger klog.Logger, obj any) {
	nsKey, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		runtime.HandleError(err)
		return
	}

	key := fmt.Sprintf("%s/%s", c.providerNamespace, nsKey)
	logger.V(2).Info("queueing APIServiceNamespace", "key", key, "reason", "Namespace", "NamespaceKey", nsKey)
	c.queue.Add(key)
}

// Start starts the controller, which stops when ctx.Done() is closed.
func (c *controller) Start(ctx context.Context, numThreads int) {
	defer runtime.HandleCrash()
	defer c.queue.ShutDown()

	logger := klog.FromContext(ctx).WithValues("controller", controllerName)

	logger.Info("Starting controller")
	defer logger.Info("Shutting down controller")

	c.namespaceInformer.Informer().AddDynamicEventHandler(ctx, controllerName, cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj any) {
			c.enqueueNamespace(logger, obj)
		},
		UpdateFunc: func(_, newObj any) {
			c.enqueueNamespace(logger, newObj)
		},
		DeleteFunc: func(obj any) {
			c.enqueueNamespace(logger, obj)
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
	logger.Info("processing key")

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
	snsNamespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(err)
		return nil // we cannot do anything
	}

	current, err := c.getServiceNamespace(snsNamespace, name)
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	switch {
	case errors.IsNotFound(err):
		return c.handleNamespaceDeletion(ctx, snsNamespace, name)
	default:
		return c.handleNamespaceLifecycle(ctx, current)
	}
}

// handleNamespaceDeletion ensures that when an APIServiceNamespace is deleted,
// the corresponding Namespace is also deleted, does not matter who owns it.
func (c *controller) handleNamespaceDeletion(ctx context.Context, snsNamespace, name string) error {
	logger := klog.FromContext(ctx).WithValues("APIServiceNamespace", fmt.Sprintf("%s/%s", snsNamespace, name))
	logger.Info("handling deletion of APIServiceNamespace")

	_, err := c.getNamespace(name)
	if err != nil && !errors.IsNotFound(err) {
		return err
	} else if errors.IsNotFound(err) {
		// Namespace already deleted, lets check who owns
		return nil
	}
	// We should not delete namespaces in this case as we can't be sure who owns it.
	// This might leave hanging namespaces, but it's safer than deleting something we don't own.
	logger.Info("not deleting Namespace as we can't be sure who owns it")
	return nil
}

func (c *controller) handleNamespaceLifecycle(ctx context.Context, current *kubebindv1alpha2.APIServiceNamespace) error {
	logger := klog.FromContext(ctx).WithValues("APIServiceNamespace", fmt.Sprintf("%s/%s", current.Namespace, current.Name))
	logger.Info("handling creation of APIServiceNamespace")

	// At this point we know that the APIServiceNamespace exists, and we need to see who owns it and
	// act accordingly.
	// If provider owned, ensure Namespace exists. If consumer owned, and local namespace exists, delete it.
	consumerOwned := isConsumerOwned(current)

	switch consumerOwned {
	case true:
		// Consumer owned, ensure Namespace is deleted if exists
		_, err := c.getNamespace(current.Name)
		if errors.IsNotFound(err) {
			// No local namespace - delete APIServiceNamespace if exists.
			return c.deleteServiceNamespace(ctx, current.Namespace, current.Name)
		} else if err != nil {
			return err
		}
	case false:
		_, err := c.getNamespace(current.Name)
		if errors.IsNotFound(err) {
			ns := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: current.Name,
					Labels: map[string]string{
						kubebindv1alpha2.ObjectOwnerLabel: kubebindv1alpha2.OwnerProvider.String(),
					},
				},
			}
			if _, err := c.consumerKubeClient.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{}); err != nil {
				return err
			}
			logger.Info("created Namespace for APIServiceNamespace")
			return nil
		} else if err != nil {
			return err
		}
	}
	return nil
}

// isConsumerOwned checks if the APIServiceNamespace is owned by consumer.
// If the label is missing, it is considered consumer owned.
func isConsumerOwned(sns *kubebindv1alpha2.APIServiceNamespace) bool {
	switch sns.Labels[kubebindv1alpha2.ObjectOwnerLabel] {
	case kubebindv1alpha2.OwnerProvider.String():
		return false
	default:
		return true
	}
}
