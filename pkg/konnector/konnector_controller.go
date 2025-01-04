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

package konnector

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	crdinformers "k8s.io/apiextensions-apiserver/pkg/client/informers/externalversions/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	coreinformers "k8s.io/client-go/informers/core/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	"github.com/kube-bind/kube-bind/pkg/committer"
	"github.com/kube-bind/kube-bind/pkg/indexers"
	"github.com/kube-bind/kube-bind/pkg/konnector/controllers/cluster"
	"github.com/kube-bind/kube-bind/pkg/konnector/controllers/dynamic"
	"github.com/kube-bind/kube-bind/pkg/konnector/controllers/servicebinding"
	kubebindv1alpha1 "github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha1"
	bindclient "github.com/kube-bind/kube-bind/sdk/client/clientset/versioned"
	bindinformers "github.com/kube-bind/kube-bind/sdk/client/informers/externalversions/kubebind/v1alpha1"
	bindlisters "github.com/kube-bind/kube-bind/sdk/client/listers/kubebind/v1alpha1"
)

const (
	controllerName = "kube-bind-konnector"
)

// New returns a konnector controller.
func New(
	consumerConfig *rest.Config,
	serviceBindingInformer bindinformers.APIServiceBindingInformer,
	secretInformer coreinformers.SecretInformer,
	namespaceInformer coreinformers.NamespaceInformer,
	crdInformer crdinformers.CustomResourceDefinitionInformer,
) (*Controller, error) {
	queue := workqueue.NewTypedRateLimitingQueueWithConfig(workqueue.DefaultTypedControllerRateLimiter[string](), workqueue.TypedRateLimitingQueueConfig[string]{Name: controllerName})

	logger := klog.Background().WithValues("Controller", controllerName)

	consumerConfig = rest.CopyConfig(consumerConfig)
	consumerConfig = rest.AddUserAgent(consumerConfig, controllerName)

	bindClient, err := bindclient.NewForConfig(consumerConfig)
	if err != nil {
		return nil, err
	}

	servicebindingCtrl, err := servicebinding.NewController(consumerConfig, serviceBindingInformer, secretInformer)
	if err != nil {
		return nil, err
	}

	namespaceDynamicInformer, err := dynamic.NewDynamicInformer(namespaceInformer)
	if err != nil {
		return nil, err
	}
	serviceBindingDynamicInformer, err := dynamic.NewDynamicInformer(serviceBindingInformer)
	if err != nil {
		return nil, err
	}
	crdDynamicInformer, err := dynamic.NewDynamicInformer(crdInformer)
	if err != nil {
		return nil, err
	}

	c := &Controller{
		queue: queue,

		consumerConfig: consumerConfig,
		bindClient:     bindClient,

		serviceBindingLister:  serviceBindingInformer.Lister(),
		serviceBindingIndexer: serviceBindingInformer.Informer().GetIndexer(),

		secretLister:  secretInformer.Lister(),
		secretIndexer: secretInformer.Informer().GetIndexer(),

		ServiceBindingCtrl: servicebindingCtrl,

		reconciler: reconciler{
			controllers: map[string]*controllerContext{},
			getSecret: func(ns, name string) (*corev1.Secret, error) {
				return secretInformer.Lister().Secrets(ns).Get(name)
			},
			newClusterController: func(consumerSecretRefKey, providerNamespace string, reconcileServiceBinding func(binding *kubebindv1alpha1.APIServiceBinding) bool, providerConfig *rest.Config) (startable, error) {
				providerConfig = rest.CopyConfig(providerConfig)
				providerConfig = rest.AddUserAgent(providerConfig, controllerName)

				return cluster.NewController(
					consumerSecretRefKey,
					providerNamespace,
					reconcileServiceBinding,
					consumerConfig,
					providerConfig,
					namespaceDynamicInformer,
					serviceBindingDynamicInformer,
					crdDynamicInformer,
				)
			},
		},

		commit: committer.NewCommitter[*kubebindv1alpha1.APIServiceBinding, *kubebindv1alpha1.APIServiceBindingSpec, *kubebindv1alpha1.APIServiceBindingStatus](
			func(ns string) committer.Patcher[*kubebindv1alpha1.APIServiceBinding] {
				return bindClient.KubeBindV1alpha1().APIServiceBindings()
			},
		),
	}

	indexers.AddIfNotPresentOrDie(serviceBindingInformer.Informer().GetIndexer(), cache.Indexers{
		indexers.ByServiceBindingKubeconfigSecret: indexers.IndexServiceBindingByKubeconfigSecret,
	})

	indexers.AddIfNotPresentOrDie(crdInformer.Informer().GetIndexer(), cache.Indexers{
		indexers.CRDByServiceBinding: indexers.IndexCRDByServiceBinding,
	})

	if _, err := serviceBindingInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			c.enqueueServiceBinding(logger, obj)
		},
		UpdateFunc: func(_, newObj interface{}) {
			c.enqueueServiceBinding(logger, newObj)
		},
		DeleteFunc: func(obj interface{}) {
			c.enqueueServiceBinding(logger, obj)
		},
	}); err != nil {
		return nil, err
	}

	if _, err := secretInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			c.enqueueSecret(logger, obj)
		},
		UpdateFunc: func(_, newObj interface{}) {
			c.enqueueSecret(logger, newObj)
		},
		DeleteFunc: func(obj interface{}) {
			c.enqueueSecret(logger, obj)
		},
	}); err != nil {
		return nil, err
	}

	return c, nil
}

type Resource = committer.Resource[*kubebindv1alpha1.APIServiceBindingSpec, *kubebindv1alpha1.APIServiceBindingStatus]
type CommitFunc = func(context.Context, *Resource, *Resource) error

type GenericController interface {
	Start(ctx context.Context, numThreads int)
}

// Controller is the top-level Controller watching ServiceBindings and
// service provider credentials, and then starts APIServiceBinding controllers
// dynamically.
type Controller struct {
	queue workqueue.TypedRateLimitingInterface[string]

	consumerConfig *rest.Config
	bindClient     bindclient.Interface

	serviceBindingLister  bindlisters.APIServiceBindingLister
	serviceBindingIndexer cache.Indexer

	secretLister  corelisters.SecretLister
	secretIndexer cache.Indexer

	ServiceBindingCtrl GenericController

	reconciler

	commit CommitFunc
}

func (c *Controller) enqueueServiceBinding(logger klog.Logger, obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		runtime.HandleError(err)
		return
	}

	logger.V(2).Info("queueing APIServiceBinding", "key", key)
	c.queue.Add(key)
}

func (c *Controller) enqueueSecret(logger klog.Logger, obj interface{}) {
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

	bindings, err := c.serviceBindingIndexer.ByIndex(indexers.ByServiceBindingKubeconfigSecret, fmt.Sprintf("%s/%s", ns, name))
	if err != nil {
		runtime.HandleError(err)
		return
	}
	if len(bindings) == 0 {
		return
	}

	for _, obj := range bindings {
		bindingKey, err := cache.MetaNamespaceKeyFunc(obj)
		if err != nil {
			runtime.HandleError(err)
			continue
		}
		logger.V(2).Info("queueing APIServiceBinding", "key", bindingKey, "reason", "Secret", "SecretKey", key)
		c.queue.Add(bindingKey)
	}
}

// Start starts the konnector. It does block.
func (c *Controller) Start(ctx context.Context, numThreads int) {
	defer runtime.HandleCrash()
	defer c.queue.ShutDown()

	logger := klog.FromContext(ctx).WithValues("Controller", controllerName)

	logger.Info("Starting Controller")
	defer logger.Info("Shutting down Controller")

	for i := 0; i < numThreads; i++ {
		go wait.UntilWithContext(ctx, c.startWorker, time.Second)
	}

	go c.ServiceBindingCtrl.Start(ctx, numThreads)

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
	logger.Info("processing key")

	// No matter what, tell the queue we're done with this key, to unblock
	// other workers.
	defer c.queue.Done(key)

	if err := c.process(ctx, key); err != nil {
		runtime.HandleError(fmt.Errorf("%q Controller failed to sync %q, err: %w", controllerName, key, err))
		c.queue.AddRateLimited(key)
		return true
	}
	c.queue.Forget(key)
	return true
}

func (c *Controller) process(ctx context.Context, key string) error {
	_, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(err)
		return nil // we cannot do anything
	}

	obj, err := c.serviceBindingLister.Get(name)
	if err != nil && !errors.IsNotFound(err) {
		return err
	} else if errors.IsNotFound(err) {
		// update remote condition
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
