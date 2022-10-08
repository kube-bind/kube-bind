/*
Copyright 2022 The KCP Authors.

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

package servicenamespace

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	coreinformers "k8s.io/client-go/informers/core/v1"
	kubernetesclient "k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	kubebindv1alpha1 "github.com/kube-bind/kube-bind/pkg/apis/kubebind/v1alpha1"
	bindclient "github.com/kube-bind/kube-bind/pkg/client/clientset/versioned"
	bindinformers "github.com/kube-bind/kube-bind/pkg/client/informers/externalversions/kubebind/v1alpha1"
	bindlisters "github.com/kube-bind/kube-bind/pkg/client/listers/kubebind/v1alpha1"
)

const (
	controllerName = "servicenamespace"
)

// NewController returns a new controller for ServiceNamespaces.
func NewController(
	config *rest.Config,
	serviceNamespaceInformer bindinformers.ServiceNamespaceInformer,
	namespaceInformer coreinformers.NamespaceInformer,
) (*controller, error) {
	queue := workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), controllerName)

	config = rest.CopyConfig(config)
	config = rest.AddUserAgent(config, controllerName)

	bindClient, err := bindclient.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	kubeClient, err := kubernetesclient.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	c := &controller{
		queue: queue,

		bindClient: bindClient,
		kubeClient: kubeClient,

		serviceNamespaceLister:  serviceNamespaceInformer.Lister(),
		serviceNamespaceIndexer: serviceNamespaceInformer.Informer().GetIndexer(),

		namespaceLister:  namespaceInformer.Lister(),
		namespaceIndexer: namespaceInformer.Informer().GetIndexer(),

		getNamespace: namespaceInformer.Lister().Get,
		createNamespace: func(ctx context.Context, ns *corev1.Namespace) error {
			_, err := kubeClient.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
			return err
		},
		deleteNamespace: func(ctx context.Context, name string) error {
			return kubeClient.CoreV1().Namespaces().Delete(ctx, name, metav1.DeleteOptions{})
		},

		getServiceNamespace: func(ns, name string) (*kubebindv1alpha1.ServiceNamespace, error) {
			return serviceNamespaceInformer.Lister().ServiceNamespaces(ns).Get(name)
		},
	}

	// nolint:errcheck
	namespaceInformer.Informer().GetIndexer().AddIndexers(cache.Indexers{
		ByServiceNamespace: IndexByServiceNamespace,
	})

	namespaceInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			c.enqueueNamespace(obj)
		},
		UpdateFunc: func(_, newObj interface{}) {
			c.enqueueNamespace(newObj)
		},
		DeleteFunc: func(obj interface{}) {
			c.enqueueNamespace(obj)
		},
	})

	serviceNamespaceInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			c.enqueueServiceNamespace(obj)
		},
		UpdateFunc: func(_, newObj interface{}) {
			c.enqueueServiceNamespace(newObj)
		},
		DeleteFunc: func(obj interface{}) {
			c.enqueueServiceNamespace(obj)
		},
	})

	return c, nil
}

// controller reconciles ServiceNamespaces by creating a Namespace for each, and deleting it if
// the ServiceNamespace is deleted.
type controller struct {
	queue workqueue.RateLimitingInterface

	bindClient bindclient.Interface
	kubeClient kubernetesclient.Interface

	namespaceLister  corelisters.NamespaceLister
	namespaceIndexer cache.Indexer

	serviceNamespaceLister  bindlisters.ServiceNamespaceLister
	serviceNamespaceIndexer cache.Indexer

	getNamespace    func(name string) (*corev1.Namespace, error)
	createNamespace func(ctx context.Context, ns *corev1.Namespace) error
	deleteNamespace func(ctx context.Context, name string) error

	getServiceNamespace func(ns, name string) (*kubebindv1alpha1.ServiceNamespace, error)
}

// enqueueAPIBinding enqueues an APIExport .
func (c *controller) enqueueServiceNamespace(obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		runtime.HandleError(err)
		return
	}

	klog.V(2).InfoS("queueing APIExport", "controller", controllerName, "key", key)
	c.queue.Add(key)
}

func (c *controller) enqueueNamespace(obj interface{}) {
	if ns, ok := obj.(*corev1.Namespace); ok {
		if value, found := ns.Annotations[serviceNamespaceAnnotationKey]; found {
			ns, name, err := ServiceNamespaceFromAnnotation(value)
			if err != nil {
				runtime.HandleError(err)
				return
			}
			key := ns + "/" + name

			klog.V(2).InfoS("queueing ServiceNamespace via namespace annotation", "controller", controllerName, "key", key)
			c.queue.Add(key)
		}
	}
}

// Start starts the controller, which stops when ctx.Done() is closed.
func (c *controller) Start(ctx context.Context, numThreads int) {
	defer runtime.HandleCrash()
	defer c.queue.ShutDown()

	klog.InfoS("Starting controller", "controller", controllerName)
	defer klog.InfoS("Shutting down controller", "controller", controllerName)

	for i := 0; i < numThreads; i++ {
		go wait.UntilWithContext(ctx, c.startWorker, time.Second)
	}

	<-ctx.Done()
}

func (c *controller) startWorker(ctx context.Context) {
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

	klog.V(2).InfoS("processing key", "controller", controllerName, "key", key)

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
	snsNamespace, snsName, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(err)
		return nil // we cannot do anything
	}
	nsName := snsNamespace + "-" + snsName

	if _, err := c.getServiceNamespace(snsNamespace, snsName); err != nil && !errors.IsNotFound(err) {
		return err
	} else if errors.IsNotFound(err) {
		if err := c.deleteNamespace(ctx, nsName); err != nil && !errors.IsNotFound(err) {
			return err
		}
		return nil
	}

	if _, err := c.getNamespace(nsName); err != nil && !errors.IsNotFound(err) {
		return err // should not happen
	}
	if errors.IsNotFound(err) {
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: nsName,
				Annotations: map[string]string{
					serviceNamespaceAnnotationKey: ServiceNamespaceAnnotationValue(snsNamespace, snsName),
				},
			},
		}
		if err := c.createNamespace(ctx, ns); err != nil {
			return fmt.Errorf("failed to create namespace %q: %w", nsName, err)
		}
	}

	return nil
}
