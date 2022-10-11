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

package servicebinding

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apiextensionsinformers "k8s.io/apiextensions-apiserver/pkg/client/informers/externalversions/apiextensions/v1"
	apiextensionslisters "k8s.io/apiextensions-apiserver/pkg/client/listers/apiextensions/v1"
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

	kubebindv1alpha1 "github.com/kube-bind/kube-bind/pkg/apis/kubebind/v1alpha1"
	bindclient "github.com/kube-bind/kube-bind/pkg/client/clientset/versioned"
	bindinformers "github.com/kube-bind/kube-bind/pkg/client/informers/externalversions/kubebind/v1alpha1"
	bindlisters "github.com/kube-bind/kube-bind/pkg/client/listers/kubebind/v1alpha1"
	"github.com/kube-bind/kube-bind/pkg/committer"
	"github.com/kube-bind/kube-bind/pkg/indexers"
)

const (
	controllerName = "kube-bind-konnector-servicebinding"
)

// NewController returns a new controller for ServiceBindings.
func NewController(
	consumerConfig *rest.Config,
	serviceBindingInformer bindinformers.ServiceBindingInformer,
	consumerSecretInformer coreinformers.SecretInformer,
	crdInformer apiextensionsinformers.CustomResourceDefinitionInformer,
) (*controller, error) {
	queue := workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), controllerName)

	logger := klog.Background().WithValues("controller", controllerName)

	consumerConfig = rest.CopyConfig(consumerConfig)
	consumerConfig = rest.AddUserAgent(consumerConfig, controllerName)

	consumerBindClient, err := bindclient.NewForConfig(consumerConfig)
	if err != nil {
		return nil, err
	}

	c := &controller{
		queue: queue,

		serviceBindingLister:  serviceBindingInformer.Lister(),
		serviceBindingIndexer: serviceBindingInformer.Informer().GetIndexer(),

		consumerSecretLister: consumerSecretInformer.Lister(),

		crdLister:  crdInformer.Lister(),
		crdIndexer: crdInformer.Informer().GetIndexer(),

		reconciler: reconciler{
			getConsumerSecret: func(ns, name string) (*corev1.Secret, error) {
				return consumerSecretInformer.Lister().Secrets(ns).Get(name)
			},
		},

		commit: committer.NewCommitter[*kubebindv1alpha1.ServiceBinding, *kubebindv1alpha1.ServiceBindingSpec, *kubebindv1alpha1.ServiceBindingStatus](
			func(ns string) committer.Patcher[*kubebindv1alpha1.ServiceBinding] {
				return consumerBindClient.KubeBindV1alpha1().ServiceBindings()
			},
		),
	}

	// nolint:errcheck
	indexers.AddIfNotPresentOrDie(serviceBindingInformer.Informer().GetIndexer(), cache.Indexers{
		indexers.ByServiceBindingKubeconfigSecret: indexers.IndexServiceBindingByKubeconfigSecret,
	})

	serviceBindingInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
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

	consumerSecretInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			c.enqueueConsumerSecret(logger, obj)
		},
		UpdateFunc: func(_, newObj interface{}) {
			c.enqueueConsumerSecret(logger, newObj)
		},
		DeleteFunc: func(obj interface{}) {
			c.enqueueConsumerSecret(logger, obj)
		},
	})

	consumerSecretInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			c.enqueueConsumerSecret(logger, obj)
		},
		UpdateFunc: func(_, newObj interface{}) {
			c.enqueueConsumerSecret(logger, newObj)
		},
		DeleteFunc: func(obj interface{}) {
			c.enqueueConsumerSecret(logger, obj)
		},
	})

	return c, nil
}

type Resource = committer.Resource[*kubebindv1alpha1.ServiceBindingSpec, *kubebindv1alpha1.ServiceBindingStatus]
type CommitFunc = func(context.Context, *Resource, *Resource) error

// controller reconciles ServiceBindings' kubeconfig secret references. It is
// here as an individual controller because the cluster controller is not running
// if the secret is invalid.
type controller struct {
	queue workqueue.RateLimitingInterface

	serviceBindingLister  bindlisters.ServiceBindingLister
	serviceBindingIndexer cache.Indexer

	consumerSecretLister corelisters.SecretLister

	crdLister  apiextensionslisters.CustomResourceDefinitionLister
	crdIndexer cache.Indexer

	reconciler

	commit CommitFunc
}

func (c *controller) enqueueServiceBinding(logger klog.Logger, obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		runtime.HandleError(err)
		return
	}

	logger.V(2).Info("queueing ServiceBinding", "key", key)
	c.queue.Add(key)
}

func (c *controller) enqueueConsumerSecret(logger klog.Logger, obj interface{}) {
	secretKey, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		runtime.HandleError(err)
		return
	}

	bindings, err := c.serviceBindingIndexer.ByIndex(indexers.ByServiceBindingKubeconfigSecret, secretKey)
	if err != nil && !errors.IsNotFound(err) {
		runtime.HandleError(err)
		return
	} else if errors.IsNotFound(err) {
		return // skip this secret
	}

	for _, obj := range bindings {
		binding := obj.(*kubebindv1alpha1.ServiceBinding)
		key, err := cache.MetaNamespaceKeyFunc(binding)
		if err != nil {
			runtime.HandleError(err)
			return
		}
		logger.V(2).Info("queueing ServiceBinding", "key", key, "reason", "Secret", "SecretKey", secretKey)
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
	_, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(err)
		return nil // we cannot do anything
	}

	logger := klog.FromContext(ctx)

	obj, err := c.serviceBindingLister.Get(name)
	if err != nil && !errors.IsNotFound(err) {
		return err
	} else if errors.IsNotFound(err) {
		logger.Error(err, "ServiceBinding disappeared")
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
