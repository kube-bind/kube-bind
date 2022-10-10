/*
Copyright 2022 The kube bind Authors.

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
	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
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

	kubebindv1alpha1 "github.com/kube-bind/kube-bind/pkg/apis/kubebind/v1alpha1"
	bindclient "github.com/kube-bind/kube-bind/pkg/client/clientset/versioned"
	bindinformers "github.com/kube-bind/kube-bind/pkg/client/informers/externalversions/kubebind/v1alpha1"
	bindlisters "github.com/kube-bind/kube-bind/pkg/client/listers/kubebind/v1alpha1"
	"github.com/kube-bind/kube-bind/pkg/committer"
	"github.com/kube-bind/kube-bind/pkg/indexers"
	"github.com/kube-bind/kube-bind/pkg/konnector/controllers/dynamic"
)

const (
	controllerName = "kube-bind-konnector-cluster-serviceexport"
)

// NewController returns a new controller for ClusterBindings.
func NewController(
	consumerSecretRefKey, providerNamespace string,
	consumerConfig, providerConfig *rest.Config,
	serviceExportInformer bindinformers.ServiceExportInformer,
	serviceExportResourceInformer bindinformers.ServiceExportResourceInformer,
	serviceBindingInformer dynamic.Informer[bindlisters.ServiceBindingLister],
	crdInformer dynamic.Informer[apiextensionslisters.CustomResourceDefinitionLister],
) (*controller, error) {
	queue := workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), controllerName)

	logger := klog.Background().WithValues("controller", controllerName)

	providerConfig = rest.CopyConfig(providerConfig)
	providerConfig = rest.AddUserAgent(providerConfig, controllerName)

	consumerConfig = rest.CopyConfig(consumerConfig)
	consumerConfig = rest.AddUserAgent(consumerConfig, controllerName)

	providerBindClient, err := bindclient.NewForConfig(providerConfig)
	if err != nil {
		return nil, err
	}
	apiextensionsClient, err := apiextensionsclient.NewForConfig(consumerConfig)
	if err != nil {
		return nil, err
	}

	c := &controller{
		queue: queue,

		consumerSecretRefKey: consumerSecretRefKey,
		providerNamespace:    providerNamespace,

		providerBindClient:  providerBindClient,
		apiextensionsClient: apiextensionsClient,

		serviceExportLister:  serviceExportInformer.Lister(),
		serviceExportIndexer: serviceExportInformer.Informer().GetIndexer(),

		serviceExportResourceLister:  serviceExportResourceInformer.Lister(),
		serviceExportResourceIndexer: serviceExportResourceInformer.Informer().GetIndexer(),

		serviceBindingInformer: serviceBindingInformer,
		crdInformer:            crdInformer,

		reconciler: reconciler{
			listServiceBinding: func(export string) ([]*kubebindv1alpha1.ServiceBinding, error) {
				objs, err := serviceBindingInformer.Informer().GetIndexer().ByIndex(indexers.ByKubeconfigSecret, consumerSecretRefKey)
				if err != nil {
					return nil, err
				}
				var bindings []*kubebindv1alpha1.ServiceBinding
				for _, obj := range objs {
					binding := obj.(*kubebindv1alpha1.ServiceBinding)
					if binding.Spec.Export == export {
						bindings = append(bindings, binding)
					}
				}
				return bindings, nil
			},
			getCRD: func(name string) (*apiextensionsv1.CustomResourceDefinition, error) {
				return crdInformer.Lister().Get(name)
			},
			updateCRD: func(ctx context.Context, crd *apiextensionsv1.CustomResourceDefinition) (*apiextensionsv1.CustomResourceDefinition, error) {
				return apiextensionsClient.ApiextensionsV1().CustomResourceDefinitions().Update(ctx, crd, metav1.UpdateOptions{})
			},
			createCRD: func(ctx context.Context, crd *apiextensionsv1.CustomResourceDefinition) (*apiextensionsv1.CustomResourceDefinition, error) {
				return apiextensionsClient.ApiextensionsV1().CustomResourceDefinitions().Create(ctx, crd, metav1.CreateOptions{})
			},
			getServiceExportResource: func(name string) (*kubebindv1alpha1.ServiceExportResource, error) {
				return serviceExportResourceInformer.Lister().ServiceExportResources(providerNamespace).Get(name)
			},
			updateServiceExportResourceStatus: func(ctx context.Context, resource *kubebindv1alpha1.ServiceExportResource) (*kubebindv1alpha1.ServiceExportResource, error) {
				return providerBindClient.KubeBindV1alpha1().ServiceExportResources(providerNamespace).UpdateStatus(ctx, resource, metav1.UpdateOptions{})
			},
		},

		commit: committer.NewCommitter[*kubebindv1alpha1.ServiceExport, *kubebindv1alpha1.ServiceExportSpec, *kubebindv1alpha1.ServiceExportStatus](
			func(ns string) committer.Patcher[*kubebindv1alpha1.ServiceExport] {
				return providerBindClient.KubeBindV1alpha1().ServiceExports(ns)
			},
		),
	}

	indexers.AddIfNotPresentOrDie(serviceExportInformer.Informer().GetIndexer(), cache.Indexers{
		ByGroupResource: IndexByGroupResource,
	})

	serviceExportInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			c.enqueueServiceExport(logger, obj)
		},
		UpdateFunc: func(_, newObj interface{}) {
			c.enqueueServiceExport(logger, newObj)
		},
		DeleteFunc: func(obj interface{}) {
			c.enqueueServiceExport(logger, obj)
		},
	})

	serviceExportResourceInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			c.enqueueServiceExportResource(logger, obj)
		},
		UpdateFunc: func(_, newObj interface{}) {
			c.enqueueServiceExportResource(logger, newObj)
		},
		DeleteFunc: func(obj interface{}) {
			c.enqueueServiceExportResource(logger, obj)
		},
	})

	return c, nil
}

type Resource = committer.Resource[*kubebindv1alpha1.ServiceExportSpec, *kubebindv1alpha1.ServiceExportStatus]
type CommitFunc = func(context.Context, *Resource, *Resource) error

// controller reconciles ClusterBindings on the service provider cluster, including heartbeating.
type controller struct {
	queue workqueue.RateLimitingInterface

	// consumerSecretRefKey is the namespace/name value of the ServiceBinding kubeconfig secret reference.
	consumerSecretRefKey string
	providerNamespace    string

	providerBindClient  bindclient.Interface
	apiextensionsClient apiextensionsclient.Interface

	serviceExportLister  bindlisters.ServiceExportLister
	serviceExportIndexer cache.Indexer

	serviceExportResourceLister  bindlisters.ServiceExportResourceLister
	serviceExportResourceIndexer cache.Indexer

	serviceBindingInformer dynamic.Informer[bindlisters.ServiceBindingLister]
	crdInformer            dynamic.Informer[apiextensionslisters.CustomResourceDefinitionLister]

	reconciler

	commit CommitFunc
}

func (c *controller) enqueueServiceExport(logger klog.Logger, obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		runtime.HandleError(err)
		return
	}

	logger.V(2).Info("queueing ServiceExport", "key", key)
	c.queue.Add(key)
}

func (c *controller) enqueueServiceBinding(logger klog.Logger, obj interface{}) {
	binding, ok := obj.(*kubebindv1alpha1.ServiceBinding)
	if !ok {
		runtime.HandleError(fmt.Errorf("unexpected type %T", obj))
		return
	}
	if indexers.ByKubeconfigSecretKey(binding) != c.consumerSecretRefKey {
		return // not for us
	}

	key := c.providerNamespace + "/" + binding.Spec.Export
	logger.V(2).Info("queueing ServiceExport", "key", key, "reason", "ServiceBinding", "ServiceBindingKey", binding.Name)
	c.queue.Add(key)
}

func (c *controller) enqueueServiceExportResource(logger klog.Logger, obj interface{}) {
	serKey, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		runtime.HandleError(err)
		return
	}

	ns, name, err := cache.SplitMetaNamespaceKey(serKey)
	if err != nil {
		runtime.HandleError(err)
		return
	}
	if ns != c.providerNamespace {
		return // not for us
	}

	exports, err := c.serviceExportIndexer.ByIndex(ByGroupResource, name)
	if err != nil {
		runtime.HandleError(err)
		return
	}
	for _, obj := range exports {
		export := obj.(*kubebindv1alpha1.ServiceExport)
		key := c.providerNamespace + "/" + export.Name
		logger.V(2).Info("queueing ServiceExport", "key", key, "reason", "ServiceExportResource", "ServiceExportResourceKey", serKey)
		c.queue.Add(key)
	}
}

func (c *controller) enqueueCRD(logger klog.Logger, obj interface{}) {
	name, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		runtime.HandleError(err)
		return
	}

	exports, err := c.serviceExportIndexer.ByIndex(ByGroupResource, name)
	if err != nil {
		runtime.HandleError(err)
		return
	}
	for _, obj := range exports {
		export := obj.(*kubebindv1alpha1.ServiceExport)
		key := c.providerNamespace + "/" + export.Name
		logger.V(2).Info("queueing ServiceExport", "key", key, "reason", "CustomResourceDefinition", "CustomResourceDefinitionKey", name)
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

	c.serviceBindingInformer.Informer().AddDynamicEventHandler(ctx, controllerName, cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			c.enqueueServiceBinding(logger, obj)
		},
		UpdateFunc: func(_, newObj interface{}) {
			c.enqueueServiceBinding(logger, newObj)
		},
		DeleteFunc: func(obj interface{}) {
			c.enqueueServiceBinding(logger, obj)
		},
	})

	c.crdInformer.Informer().AddDynamicEventHandler(ctx, controllerName, cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			c.enqueueCRD(logger, obj)
		},
		UpdateFunc: func(_, newObj interface{}) {
			c.enqueueCRD(logger, newObj)
		},
		DeleteFunc: func(obj interface{}) {
			c.enqueueCRD(logger, obj)
		},
	})

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

	obj, err := c.serviceExportLister.ServiceExports(ns).Get(name)
	if err != nil && !errors.IsNotFound(err) {
		return err
	} else if errors.IsNotFound(err) {
		logger.Error(err, "ServiceExport disappeared")
		return nil
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
