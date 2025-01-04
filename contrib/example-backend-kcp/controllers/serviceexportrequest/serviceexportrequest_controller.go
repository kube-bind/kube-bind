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

package serviceexportrequest

import (
	"context"
	"fmt"
	"time"

	"github.com/google/go-cmp/cmp"
	kcpcache "github.com/kcp-dev/apimachinery/v2/pkg/cache"
	apiextensionsinformers "github.com/kcp-dev/client-go/apiextensions/informers/apiextensions/v1"
	apiextensionslisters "github.com/kcp-dev/client-go/apiextensions/listers/apiextensions/v1"
	"github.com/kcp-dev/logicalcluster/v3"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	kubernetesclient "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	"github.com/kube-bind/kube-bind/contrib/example-backend-kcp/committer"
	"github.com/kube-bind/kube-bind/pkg/indexers"
	kubebindv1alpha1 "github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha1"
	bindclient "github.com/kube-bind/kube-bind/sdk/kcp/clientset/versioned/cluster"
	bindinformers "github.com/kube-bind/kube-bind/sdk/kcp/informers/externalversions/kubebind/v1alpha1"
	bindlisters "github.com/kube-bind/kube-bind/sdk/kcp/listers/kubebind/v1alpha1"
)

const (
	controllerName = "kube-bind-example-backend-serviceexportrequest"
)

// NewController returns a new controller to reconcile APIServiceExportRequests by
// creating corresponding APIServiceExports.
func NewController(
	config *rest.Config,
	scope kubebindv1alpha1.Scope,
	serviceExportRequestInformer bindinformers.APIServiceExportRequestClusterInformer,
	serviceExportInformer bindinformers.APIServiceExportClusterInformer,
	crdInformer apiextensionsinformers.CustomResourceDefinitionClusterInformer,
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

		serviceExportRequestLister:  serviceExportRequestInformer.Lister(),
		serviceExportRequestIndexer: serviceExportRequestInformer.Informer().GetIndexer(),

		crdLister:  crdInformer.Lister(),
		crdIndexer: crdInformer.Informer().GetIndexer(),

		reconciler: reconciler{
			informerScope: scope,
			getCRD: func(cluster logicalcluster.Name, name string) (*apiextensionsv1.CustomResourceDefinition, error) {
				return crdInformer.Lister().Cluster(cluster).Get(name)
			},
			getServiceExport: func(cluster logicalcluster.Name, ns, name string) (*kubebindv1alpha1.APIServiceExport, error) {
				return serviceExportInformer.Lister().Cluster(cluster).APIServiceExports(ns).Get(name)
			},
			createServiceExport: func(ctx context.Context, clusterName logicalcluster.Name, resource *kubebindv1alpha1.APIServiceExport) (*kubebindv1alpha1.APIServiceExport, error) {
				return bindClient.KubeBindV1alpha1().Cluster(clusterName.Path()).APIServiceExports(resource.Namespace).Create(ctx, resource, metav1.CreateOptions{})
			},
			deleteServiceExportRequest: func(ctx context.Context, cluster logicalcluster.Name, ns, name string) error {
				return bindClient.KubeBindV1alpha1().Cluster(cluster.Path()).APIServiceExportRequests(ns).Delete(ctx, name, metav1.DeleteOptions{})
			},
		},
	}

	indexers.AddIfNotPresentOrDie(serviceExportRequestInformer.Informer().GetIndexer(), cache.Indexers{
		indexers.ServiceExportRequestByServiceExport: indexers.IndexServiceExportRequestByServiceExport,
	})
	indexers.AddIfNotPresentOrDie(serviceExportRequestInformer.Informer().GetIndexer(), cache.Indexers{
		indexers.ServiceExportRequestByGroupResource: indexers.IndexServiceExportRequestByGroupResource,
	})

	serviceExportRequestInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			c.enqueueServiceExportRequest(logger, obj)
		},
		UpdateFunc: func(old, newObj interface{}) {
			c.enqueueServiceExportRequest(logger, newObj)
		},
		DeleteFunc: func(obj interface{}) {
			c.enqueueServiceExportRequest(logger, obj)
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

	crdInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			c.enqueueCRD(logger, obj)
		},
		UpdateFunc: func(old, newObj interface{}) {
			c.enqueueCRD(logger, newObj)
		},
		DeleteFunc: func(obj interface{}) {
			c.enqueueCRD(logger, obj)
		},
	})

	return c, nil
}

type Resource = committer.Resource[*kubebindv1alpha1.APIServiceExportRequestSpec, *kubebindv1alpha1.APIServiceExportRequestStatus]

// Controller to reconcile APIServiceExportRequests by creating corresponding APIServiceExports.
type Controller struct {
	queue workqueue.RateLimitingInterface

	bindClient bindclient.ClusterInterface
	kubeClient kubernetesclient.Interface

	serviceExportRequestLister  bindlisters.APIServiceExportRequestClusterLister
	serviceExportRequestIndexer cache.Indexer

	serviceExportLister  bindlisters.APIServiceExportClusterLister
	serviceExportIndexer cache.Indexer

	crdLister  apiextensionslisters.CustomResourceDefinitionClusterLister
	crdIndexer cache.Indexer

	reconciler
}

func (c *Controller) enqueueServiceExportRequest(logger klog.Logger, obj interface{}) {
	key, err := kcpcache.DeletionHandlingMetaClusterNamespaceKeyFunc(obj)
	if err != nil {
		runtime.HandleError(err)
		return
	}

	logger.V(2).Info("queueing APIServiceExportRequest", "key", key)
	c.queue.Add(key)
}

func (c *Controller) enqueueServiceExport(logger klog.Logger, obj interface{}) {
	seKey, err := kcpcache.DeletionHandlingMetaClusterNamespaceKeyFunc(obj)
	if err != nil {
		runtime.HandleError(err)
		return
	}

	requests, err := c.serviceExportRequestIndexer.ByIndex(indexers.ServiceExportRequestByServiceExport, seKey)
	if err != nil {
		runtime.HandleError(err)
		return
	}
	for _, obj := range requests {
		key, err := kcpcache.MetaClusterNamespaceKeyFunc(obj)
		if err != nil {
			runtime.HandleError(err)
			continue
		}
		logger.V(2).Info("queueing APIServiceExportRequest", "key", key, "reason", "APIServiceExport", "APIServiceExportKey", seKey)
		c.queue.Add(key)
	}
}

func (c *Controller) enqueueCRD(logger klog.Logger, obj interface{}) {
	crdKey, err := kcpcache.DeletionHandlingMetaClusterNamespaceKeyFunc(obj)
	if err != nil {
		runtime.HandleError(err)
		return
	}

	requests, err := c.serviceExportRequestIndexer.ByIndex(indexers.ServiceExportRequestByGroupResource, crdKey)
	if err != nil {
		runtime.HandleError(err)
		return
	}
	for _, obj := range requests {
		key, err := kcpcache.MetaClusterNamespaceKeyFunc(obj)
		if err != nil {
			runtime.HandleError(err)
			continue
		}
		logger.V(2).Info("queueing APIServiceExportRequest", "key", key, "reason", "CustomResourceDefinition", "CustomResourceDefinitionKey", crdKey)
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

	clusterName, snsNamespace, snsName, err := kcpcache.SplitMetaClusterNamespaceKey(key)
	if err != nil {
		return nil
	}

	obj, err := c.serviceExportRequestLister.Cluster(clusterName).APIServiceExportRequests(snsNamespace).Get(snsName)
	if err != nil && !errors.IsNotFound(err) {
		return err
	} else if errors.IsNotFound(err) {
		logger.V(2).Info("APIServiceExport not found, ignoring")
		return nil // nothing we can do
	}

	old := obj
	obj = obj.DeepCopy()

	var errs []error
	if err := c.reconcile(ctx, clusterName, obj); err != nil {
		errs = append(errs, err)
	}

	// Regardless of whether reconcile returned an error or not, always try to patch status if needed. Return the
	// reconciliation error at the end.

	// If the object being reconciled changed as a result, update it.
	objectMetaChanged := !equality.Semantic.DeepEqual(old.ObjectMeta, obj.ObjectMeta)
	specChanged := !equality.Semantic.DeepEqual(old.Spec, obj.Spec)
	statusChanged := !equality.Semantic.DeepEqual(old.Status, obj.Status)

	specOrObjectMetaChanged := specChanged || objectMetaChanged

	// Simultaneous updates of spec and status are never allowed.
	if specOrObjectMetaChanged && statusChanged {
		panic(fmt.Sprintf("programmer error: spec and status changed in same reconcile iteration. diff=%s", cmp.Diff(old, obj)))
	}

	oldResource := &Resource{ObjectMeta: old.ObjectMeta, Spec: &old.Spec, Status: &old.Status}
	newResource := &Resource{ObjectMeta: obj.ObjectMeta, Spec: &obj.Spec, Status: &obj.Status}

	patchBytes, subresources, err := committer.GeneratePatchAndSubResources(oldResource, newResource)
	if err != nil {
		errs = append(errs, err)
	}

	if len(patchBytes) == 0 {
		return nil
	}

	_, err = c.bindClient.Cluster(clusterName.Path()).KubeBindV1alpha1().APIServiceExportRequests(snsNamespace).Patch(ctx, obj.Name, types.MergePatchType, patchBytes, metav1.PatchOptions{}, subresources...)
	return err
}