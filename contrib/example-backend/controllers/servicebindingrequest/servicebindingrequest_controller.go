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

package servicebindingrequest

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	kubernetesclient "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	kubebindv1alpha1 "github.com/kube-bind/kube-bind/pkg/apis/kubebind/v1alpha1"
	bindclient "github.com/kube-bind/kube-bind/pkg/client/clientset/versioned"
	bindinformers "github.com/kube-bind/kube-bind/pkg/client/informers/externalversions/kubebind/v1alpha1"
	bindlisters "github.com/kube-bind/kube-bind/pkg/client/listers/kubebind/v1alpha1"
	"github.com/kube-bind/kube-bind/pkg/committer"
	"github.com/kube-bind/kube-bind/pkg/indexers"
)

const (
	controllerName = "kube-bind-example-backend-servicebindingrequest"
)

// NewController returns a new controller to reconcile APIServiceBindingRequests by
// creating corresponding APIServiceExports.
func NewController(
	config *rest.Config,
	serviceBindingRequestInformer bindinformers.APIServiceBindingRequestInformer,
	serviceExportInformer bindinformers.APIServiceExportInformer,
) (*Controller, error) {
	queue := workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), controllerName)

	logger := klog.Background().WithValues("controller", controllerName)

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

	c := &Controller{
		queue: queue,

		bindClient: bindClient,
		kubeClient: kubeClient,

		serviceExportLister:  serviceExportInformer.Lister(),
		serviceExportIndexer: serviceExportInformer.Informer().GetIndexer(),

		serviceBindingRequestLister:  serviceBindingRequestInformer.Lister(),
		serviceBindingRequestIndexer: serviceBindingRequestInformer.Informer().GetIndexer(),

		reconciler: reconciler{
			deleteServiceBindingRequest: func(ctx context.Context, ns, name string) error {
				return bindClient.KubeBindV1alpha1().APIServiceBindingRequests(ns).Delete(ctx, name, metav1.DeleteOptions{})
			},
			getServiceExport: func(ns, name string) (*kubebindv1alpha1.APIServiceExport, error) {
				return serviceExportInformer.Lister().APIServiceExports(ns).Get(name)
			},
			createServiceExport: func(ctx context.Context, resource *kubebindv1alpha1.APIServiceExport) (*kubebindv1alpha1.APIServiceExport, error) {
				return bindClient.KubeBindV1alpha1().APIServiceExports(resource.Namespace).Create(ctx, resource, metav1.CreateOptions{})
			},
		},

		commit: committer.NewCommitter[*kubebindv1alpha1.APIServiceBindingRequest, *kubebindv1alpha1.APIServiceBindingRequestSpec, *kubebindv1alpha1.APIServiceBindingRequestStatus](
			func(ns string) committer.Patcher[*kubebindv1alpha1.APIServiceBindingRequest] {
				return bindClient.KubeBindV1alpha1().APIServiceBindingRequests(ns)
			},
		),
	}

	indexers.AddIfNotPresentOrDie(serviceBindingRequestInformer.Informer().GetIndexer(), cache.Indexers{
		indexers.ServiceBindingRequestByExport: indexers.IndexServiceBindingRequestByExport,
	})

	serviceBindingRequestInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			c.enqueueServiceBindingRequest(logger, obj)
		},
		UpdateFunc: func(old, newObj interface{}) {
			c.enqueueServiceBindingRequest(logger, newObj)
		},
		DeleteFunc: func(obj interface{}) {
			c.enqueueServiceBindingRequest(logger, obj)
		},
	})

	serviceExportInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			c.enqueueServiceExport(logger, obj)
		},
		UpdateFunc: func(old, newObj interface{}) {
			c.enqueueServiceExport(logger, newObj)
		},
		DeleteFunc: func(obj interface{}) {
			c.enqueueServiceExport(logger, obj)
		},
	})

	return c, nil
}

type Resource = committer.Resource[*kubebindv1alpha1.APIServiceBindingRequestSpec, *kubebindv1alpha1.APIServiceBindingRequestStatus]
type CommitFunc = func(context.Context, *Resource, *Resource) error

// Controller to reconcile APIServiceBindingRequests by creating corresponding APIServiceExports.
type Controller struct {
	queue workqueue.RateLimitingInterface

	bindClient bindclient.Interface
	kubeClient kubernetesclient.Interface

	serviceBindingRequestLister  bindlisters.APIServiceBindingRequestLister
	serviceBindingRequestIndexer cache.Indexer

	serviceExportLister  bindlisters.APIServiceExportLister
	serviceExportIndexer cache.Indexer

	reconciler

	commit CommitFunc
}

func (c *Controller) enqueueServiceBindingRequest(logger klog.Logger, obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		runtime.HandleError(err)
		return
	}

	logger.V(2).Info("queueing APIServiceBindingRequest", "key", key)
	c.queue.Add(key)
}

func (c *Controller) enqueueServiceExport(logger klog.Logger, obj interface{}) {
	seKey, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		runtime.HandleError(err)
		return
	}

	requests, err := c.serviceExportIndexer.ByIndex(indexers.ServiceBindingRequestByExport, seKey)
	if err != nil {
		runtime.HandleError(err)
		return
	}
	for _, obj := range requests {
		key, err := cache.MetaNamespaceKeyFunc(obj)
		if err != nil {
			runtime.HandleError(err)
			continue
		}
		logger.V(2).Info("queueing APIServiceBindingRequest", "key", key, "reason", "APIServiceExport", "APIServiceExportKey", seKey)
		c.queue.Add(key)
	}
}

// Start starts the controller, which stops when ctx.Done() is closed.
func (c *Controller) Start(ctx context.Context, numThreads int) {
	defer runtime.HandleCrash()
	defer c.queue.ShutDown()

	logger := klog.FromContext(ctx).WithValues("controller", controllerName)

	logger.Info("Starting controller")
	defer logger.Info("Shutting down controller")

	for i := 0; i < numThreads; i++ {
		go wait.UntilWithContext(ctx, c.startWorker, time.Second)
	}

	<-ctx.Done()
}

func (c *Controller) startWorker(ctx context.Context) {
	defer runtime.HandleCrash()

	for c.processNextWorkItem(ctx) {
	}
}

func (c *Controller) processNextWorkItem(ctx context.Context) bool {
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

func (c *Controller) process(ctx context.Context, key string) error {
	logger := klog.FromContext(ctx)

	ns, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(err)
		return nil // we cannot do anything
	}

	obj, err := c.serviceBindingRequestLister.APIServiceBindingRequests(ns).Get(name)
	if err != nil && !errors.IsNotFound(err) {
		return err
	} else if errors.IsNotFound(err) {
		logger.V(2).Info("APIServiceExport not found, ignoring")
		return nil // nothing we can do
	}

	old := obj
	obj = obj.DeepCopy()

	var errs []error
	if err := c.reconcile(ctx, obj); err != nil {
		errs = append(errs, err)
	}

	// Regardless of whether reconcile returned an error or not, always try to patch status if needed. Return the
	// reconciliation error at the end.

	// If the object being reconciled changed as a result, update it.
	oldResource := &Resource{ObjectMeta: old.ObjectMeta, Spec: &old.Spec, Status: &old.Status}
	newResource := &Resource{ObjectMeta: obj.ObjectMeta, Spec: &obj.Spec, Status: &obj.Status}
	if err := c.commit(ctx, oldResource, newResource); err != nil {
		errs = append(errs, err)
	}

	return utilerrors.NewAggregate(errs)
}
