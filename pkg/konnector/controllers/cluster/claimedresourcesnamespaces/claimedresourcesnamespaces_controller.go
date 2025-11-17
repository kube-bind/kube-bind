/*
Copyright 2025 The Kube Bind Authors.

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

package claimedresourcesnamespaces

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
	"k8s.io/client-go/informers"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	"github.com/kube-bind/kube-bind/pkg/konnector/controllers/dynamic"
	"github.com/kube-bind/kube-bind/pkg/resources"
	kubebindv1alpha2 "github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2"
	bindclientset "github.com/kube-bind/kube-bind/sdk/client/clientset/versioned"
	bindlisters "github.com/kube-bind/kube-bind/sdk/client/listers/kubebind/v1alpha2"
)

const (
	controllerName = "kube-bind-konnector-claimedresourcesnamespaces"
)

// NewController returns a new controller ensuring APIServiceNamespaces exist for claimed objects.
func NewController(
	gvr schema.GroupVersionResource,
	claim kubebindv1alpha2.PermissionClaim,
	apiServiceExport *kubebindv1alpha2.APIServiceExport,
	providerNamespace string,
	providerConfig *rest.Config,
	consumerConfig *rest.Config,
	consumerDynamicInformer informers.GenericInformer,
	serviceNamespaceInformer dynamic.Informer[bindlisters.APIServiceNamespaceLister],
	claimedResourcesEnqueueChan chan<- string,
) (*controller, error) {
	queue := workqueue.NewTypedRateLimitingQueueWithConfig(workqueue.DefaultTypedControllerRateLimiter[string](), workqueue.TypedRateLimitingQueueConfig[string]{Name: controllerName})

	logger := klog.Background().WithValues("controller", controllerName, "gvr", gvr)

	providerConfig = rest.CopyConfig(providerConfig)
	providerConfig = rest.AddUserAgent(providerConfig, controllerName)

	consumerConfig = rest.CopyConfig(consumerConfig)
	consumerConfig = rest.AddUserAgent(consumerConfig, controllerName)

	bindClient, err := bindclientset.NewForConfig(providerConfig)
	if err != nil {
		return nil, err
	}

	consumerClient, err := dynamicclient.NewForConfig(consumerConfig)
	if err != nil {
		return nil, err
	}

	c := &controller{
		queue:            queue,
		claim:            claim,
		apiServiceExport: apiServiceExport,

		providerBindClient: bindClient,
		consumerClient:     consumerClient,

		consumerDynamicInformer: consumerDynamicInformer,

		serviceNamespaceInformer: serviceNamespaceInformer,

		providerNamespace: providerNamespace,

		claimedResourcesEnqueueChan: claimedResourcesEnqueueChan,
	}

	// Only watch for ADD events on consumer side to detect when claimed objects are created
	if _, err = consumerDynamicInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			c.enqueueConsumer(logger, obj)
		},
	}); err != nil {
		return nil, err
	}

	return c, nil
}

// controller ensures APIServiceNamespaces exist for claimed objects.
type controller struct {
	queue workqueue.TypedRateLimitingInterface[string]

	claim            kubebindv1alpha2.PermissionClaim
	apiServiceExport *kubebindv1alpha2.APIServiceExport

	providerBindClient bindclientset.Interface
	consumerClient     dynamicclient.Interface

	consumerDynamicInformer informers.GenericInformer

	serviceNamespaceInformer dynamic.Informer[bindlisters.APIServiceNamespaceLister]

	providerNamespace string

	// Channel to notify claimedresources controller when new namespaces are created
	// so we don't have to wait for the next resync period.
	claimedResourcesEnqueueChan chan<- string
}

func (c *controller) isClaimed(logger klog.Logger, obj *unstructured.Unstructured, consumerSide bool) bool {
	return resources.IsClaimedWithReference(
		logger,
		obj,
		consumerSide,
		c.claim,
		c.apiServiceExport,
		c.consumerClient,
		c.serviceNamespaceInformer.Lister().APIServiceNamespaces(c.providerNamespace),
	)
}

func (c *controller) enqueueConsumer(logger klog.Logger, obj interface{}) {
	// handle tombstones
	if tombstone, ok := obj.(cache.DeletedFinalStateUnknown); ok {
		obj = tombstone.Obj
	}
	o, ok := obj.(*unstructured.Unstructured)
	if !ok {
		runtime.HandleError(fmt.Errorf("unexpected type %T in enqueueConsumer", obj))
		return
	}
	if !c.isClaimed(logger, o, true) {
		return
	}

	ns := o.GetNamespace()
	if ns == "" {
		return // cluster-scoped objects don't need namespaces
	}

	logger.V(2).Info("checking APIServiceNamespace for claimed object", "namespace", ns, "object", o.GetName())

	// Check if APIServiceNamespace already exists
	_, err := c.serviceNamespaceInformer.Lister().APIServiceNamespaces(c.providerNamespace).Get(ns)
	if err != nil {
		if errors.IsNotFound(err) {
			// No namespace - queue ServiceNamespace creation
			key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
			if err != nil {
				runtime.HandleError(err)
				return
			}
			logger.V(2).Info("queueing ServiceNamespace creation", "key", key)
			c.queue.Add(key)
			return
		}
		runtime.HandleError(fmt.Errorf("failed to get APIServiceNamespace %q: %w", ns, err))
		return
	}
	// APIServiceNamespace already exists, nothing to do
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
	ns, _, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(err)
		return nil
	}
	if ns == "" {
		return nil // we only handle namespaced objects
	}

	created, err := c.ensureServiceNamespace(ctx, ns)
	if err != nil {
		return err
	}
	if created {
		// Notify claimedresources controller that a new namespace was created
		select {
		case c.claimedResourcesEnqueueChan <- key:
		default:
			// Don't block if the channel is full
		}
	}
	return nil
}

func (c *controller) createServiceNamespace(ctx context.Context, sns *kubebindv1alpha2.APIServiceNamespace) (*kubebindv1alpha2.APIServiceNamespace, error) {
	return c.providerBindClient.KubeBindV1alpha2().APIServiceNamespaces(sns.Namespace).Create(ctx, sns, metav1.CreateOptions{})
}

// ensureServiceNamespace ensures that the APIServiceNamespace for the given object namespace exists.
func (c *controller) ensureServiceNamespace(ctx context.Context, ns string) (created bool, err error) {
	logger := klog.FromContext(ctx).WithValues("namespace", ns)
	logger.Info("ensuring APIServiceNamespace exists", "providerNamespace", c.providerNamespace, "namespace", ns)

	_, err = c.serviceNamespaceInformer.Lister().APIServiceNamespaces(c.providerNamespace).Get(ns)
	if err == nil {
		logger.V(2).Info("APIServiceNamespace already exists")
		return false, nil // already exists
	}
	if !errors.IsNotFound(err) {
		return false, err
	}

	logger.Info("creating APIServiceNamespace")
	_, err = c.createServiceNamespace(ctx, &kubebindv1alpha2.APIServiceNamespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ns,
			Namespace: c.providerNamespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(c.apiServiceExport, kubebindv1alpha2.SchemeGroupVersion.WithKind("APIServiceExport")),
			},
		},
	})
	if err != nil && !errors.IsAlreadyExists(err) {
		return false, fmt.Errorf("failed to create APIServiceNamespace %q: %w", ns, err)
	}

	// Wait until the servicenamespace status has been updated.
	// As this controller does not do anything else, we are ok to block here.
	err = wait.PollUntilContextCancel(ctx, 500*time.Millisecond, true, func(ctx context.Context) (done bool, err error) {
		sns, err := c.serviceNamespaceInformer.Lister().APIServiceNamespaces(c.providerNamespace).Get(ns)
		if err != nil {
			if errors.IsNotFound(err) {
				return false, nil // keep polling
			}
			return false, err
		}
		if sns.Status.Namespace != "" {
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		return false, fmt.Errorf("failed to wait for APIServiceNamespace %q to be ready: %w", ns, err)
	}

	logger.Info("APIServiceNamespace created successfully")
	return true, nil
}
