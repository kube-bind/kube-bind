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

package serviceexport

import (
	"context"
	"fmt"
	"time"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsinformers "k8s.io/apiextensions-apiserver/pkg/client/informers/externalversions/apiextensions/v1"
	apiextensionslisters "k8s.io/apiextensions-apiserver/pkg/client/listers/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	"github.com/kube-bind/kube-bind/pkg/committer"
	"github.com/kube-bind/kube-bind/pkg/indexers"
	kubebindv1alpha2 "github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2"
	bindclient "github.com/kube-bind/kube-bind/sdk/client/clientset/versioned"
	bindinformers "github.com/kube-bind/kube-bind/sdk/client/informers/externalversions/kubebind/v1alpha2"
	bindlisters "github.com/kube-bind/kube-bind/sdk/client/listers/kubebind/v1alpha2"
)

const (
	controllerName = "kube-bind-example-backend-serviceexport"
)

// NewController returns a new controller to reconcile ServiceExports.
func NewController(
	config *rest.Config,
	serviceExportInformer bindinformers.APIServiceExportInformer,
	crdInformer apiextensionsinformers.CustomResourceDefinitionInformer,
) (*Controller, error) {
	queue := workqueue.NewTypedRateLimitingQueueWithConfig(workqueue.DefaultTypedControllerRateLimiter[string](), workqueue.TypedRateLimitingQueueConfig[string]{Name: controllerName})

	logger := klog.Background().WithValues("controller", controllerName)

	config = rest.CopyConfig(config)
	config = rest.AddUserAgent(config, controllerName)

	bindClient, err := bindclient.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	c := &Controller{
		queue: queue,

		bindClient: bindClient,

		serviceExportLister:  serviceExportInformer.Lister(),
		serviceExportIndexer: serviceExportInformer.Informer().GetIndexer(),

		crdLister:  crdInformer.Lister(),
		crdIndexer: crdInformer.Informer().GetIndexer(),

		reconciler: reconciler{
			getCRD: func(name string) (*apiextensionsv1.CustomResourceDefinition, error) {
				return crdInformer.Lister().Get(name)
			},
			getAPIResourceSchema: func(ctx context.Context, name string) (*kubebindv1alpha2.APIResourceSchema, error) {
				return bindClient.KubeBindV1alpha2().APIResourceSchemas().Get(ctx, name, metav1.GetOptions{})
			},
			deleteServiceExport: func(ctx context.Context, ns, name string) error {
				return bindClient.KubeBindV1alpha2().APIServiceExports(ns).Delete(ctx, name, metav1.DeleteOptions{})
			},
			requeue: func(export *kubebindv1alpha2.APIServiceExport) {
				key, err := cache.MetaNamespaceKeyFunc(export)
				if err != nil {
					runtime.HandleError(err)
					return
				}
				queue.Add(key)
			},
		},

		commit: committer.NewCommitter[*kubebindv1alpha2.APIServiceExport, *kubebindv1alpha2.APIServiceExportSpec, *kubebindv1alpha2.APIServiceExportStatus](
			func(ns string) committer.Patcher[*kubebindv1alpha2.APIServiceExport] {
				return bindClient.KubeBindV1alpha2().APIServiceExports(ns)
			},
		),
	}

	indexers.AddIfNotPresentOrDie(serviceExportInformer.Informer().GetIndexer(), cache.Indexers{
		indexers.ServiceExportByCustomResourceDefinition: indexers.IndexServiceExportByCustomResourceDefinition,
	})

	if _, err := serviceExportInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj any) {
			c.enqueueServiceExport(logger, obj)
		},
		UpdateFunc: func(old, newObj any) {
			c.enqueueServiceExport(logger, newObj)
		},
		DeleteFunc: func(obj any) {
			c.enqueueServiceExport(logger, obj)
		},
	}); err != nil {
		return nil, err
	}

	if _, err := crdInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj any) {
			c.enqueueCRD(logger, obj)
		},
		UpdateFunc: func(old, newObj any) {
			c.enqueueCRD(logger, newObj)
		},
		DeleteFunc: func(obj any) {
			c.enqueueCRD(logger, obj)
		},
	}); err != nil {
		return nil, err
	}

	return c, nil
}

type Resource = committer.Resource[*kubebindv1alpha2.APIServiceExportSpec, *kubebindv1alpha2.APIServiceExportStatus]
type CommitFunc = func(context.Context, *Resource, *Resource) error

// Controller reconciles ServiceNamespaces by creating a Namespace for each, and deleting it if
// the APIServiceNamespace is deleted.
type Controller struct {
	queue workqueue.TypedRateLimitingInterface[string]

	bindClient bindclient.Interface

	serviceExportLister  bindlisters.APIServiceExportLister
	serviceExportIndexer cache.Indexer

	crdLister  apiextensionslisters.CustomResourceDefinitionLister
	crdIndexer cache.Indexer

	reconciler

	commit CommitFunc
}

func (c *Controller) enqueueServiceExport(logger klog.Logger, obj any) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		runtime.HandleError(err)
		return
	}

	logger.V(2).Info("queueing APIServiceExport", "key", key)
	c.queue.Add(key)
}

func (c *Controller) enqueueCRD(logger klog.Logger, obj any) {
	crdKey, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		runtime.HandleError(err)
		return
	}

	exports, err := c.serviceExportIndexer.ByIndex(indexers.ServiceExportByCustomResourceDefinition, crdKey)
	if err != nil {
		runtime.HandleError(err)
		return
	}

	for _, obj := range exports {
		export, ok := obj.(*kubebindv1alpha2.APIServiceExport)
		if !ok {
			runtime.HandleError(fmt.Errorf("unexpected type %T", obj))
			return
		}
		key, err := cache.MetaNamespaceKeyFunc(export)
		if err != nil {
			runtime.HandleError(err)
			continue
		}
		logger.V(2).Info("queueing APIServiceExport", "key", key, "reason", "CustomResourceDefinition", "CustomResourceDefinitionKey", crdKey)
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

func (c *Controller) process(ctx context.Context, key string) error {
	ns, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(err)
		return nil // we cannot do anything
	}

	obj, err := c.serviceExportLister.APIServiceExports(ns).Get(name)
	if err != nil && !errors.IsNotFound(err) {
		return err
	} else if errors.IsNotFound(err) {
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
