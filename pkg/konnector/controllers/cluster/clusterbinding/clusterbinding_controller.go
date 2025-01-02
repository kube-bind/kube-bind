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

package clusterbinding

import (
	"context"
	"fmt"
	"reflect"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	coreinformers "k8s.io/client-go/informers/core/v1"
	kubernetesclient "k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	"github.com/kube-bind/kube-bind/pkg/committer"
	"github.com/kube-bind/kube-bind/pkg/indexers"
	"github.com/kube-bind/kube-bind/pkg/konnector/controllers/dynamic"
	kubebindv1alpha1 "github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha1"
	conditionsapi "github.com/kube-bind/kube-bind/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
	"github.com/kube-bind/kube-bind/sdk/apis/third_party/conditions/util/conditions"
	bindclient "github.com/kube-bind/kube-bind/sdk/client/clientset/versioned"
	bindinformers "github.com/kube-bind/kube-bind/sdk/client/informers/externalversions/kubebind/v1alpha1"
	bindlisters "github.com/kube-bind/kube-bind/sdk/client/listers/kubebind/v1alpha1"
)

const (
	controllerName = "kube-bind-konnector-clusterbinding"
)

// NewController returns a new controller for ClusterBindings.
func NewController(
	consumerSecretRefKey string,
	providerNamespace string,
	heartbeatInterval time.Duration,
	consumerConfig, providerConfig *rest.Config,
	clusterBindingInformer bindinformers.ClusterBindingInformer,
	serviceBindingInformer dynamic.Informer[bindlisters.APIServiceBindingLister],
	serviceExportInformer bindinformers.APIServiceExportInformer,
	consumerSecretInformer, providerSecretInformer coreinformers.SecretInformer,
) (*controller, error) {
	queue := workqueue.NewTypedRateLimitingQueueWithConfig(workqueue.DefaultTypedControllerRateLimiter[string](), workqueue.TypedRateLimitingQueueConfig[string]{Name: controllerName})

	logger := klog.Background().WithValues("controller", controllerName)

	providerConfig = rest.CopyConfig(providerConfig)
	providerConfig = rest.AddUserAgent(providerConfig, controllerName)

	consumerConfig = rest.CopyConfig(consumerConfig)
	consumerConfig = rest.AddUserAgent(consumerConfig, controllerName)

	providerBindClient, err := bindclient.NewForConfig(providerConfig)
	if err != nil {
		return nil, err
	}
	providerKubeClient, err := kubernetesclient.NewForConfig(providerConfig)
	if err != nil {
		return nil, err
	}
	consumerBindClient, err := bindclient.NewForConfig(consumerConfig)
	if err != nil {
		return nil, err
	}
	consumerKubeClient, err := kubernetesclient.NewForConfig(consumerConfig)
	if err != nil {
		return nil, err
	}

	c := &controller{
		queue: queue,

		providerBindClient: providerBindClient,
		providerKubeClient: providerKubeClient,
		consumerBindClient: consumerBindClient,
		consumerKubeClient: consumerKubeClient,

		clusterBindingLister:  clusterBindingInformer.Lister(),
		clusterBindingIndexer: clusterBindingInformer.Informer().GetIndexer(),

		serviceBindingInformer: serviceBindingInformer,

		serviceExportLister:  serviceExportInformer.Lister(),
		serviceExportIndexer: serviceExportInformer.Informer().GetIndexer(),

		consumerSecretLister: consumerSecretInformer.Lister(),
		providerSecretLister: providerSecretInformer.Lister(),

		reconciler: reconciler{
			consumerSecretRefKey: consumerSecretRefKey,
			providerNamespace:    providerNamespace,
			heartbeatInterval:    heartbeatInterval,

			getProviderSecret: func() (*corev1.Secret, error) {
				cb, err := clusterBindingInformer.Lister().ClusterBindings(providerNamespace).Get("cluster")
				if err != nil {
					return nil, err
				}
				ref := &cb.Spec.KubeconfigSecretRef
				return providerSecretInformer.Lister().Secrets(providerNamespace).Get(ref.Name)
			},
			getConsumerSecret: func() (*corev1.Secret, error) {
				ns, name, err := cache.SplitMetaNamespaceKey(consumerSecretRefKey)
				if err != nil {
					return nil, err
				}
				return consumerSecretInformer.Lister().Secrets(ns).Get(name)
			},
			createConsumerSecret: func(ctx context.Context, secret *corev1.Secret) (*corev1.Secret, error) {
				return consumerKubeClient.CoreV1().Secrets(secret.Namespace).Create(ctx, secret, metav1.CreateOptions{})
			},
			updateConsumerSecret: func(ctx context.Context, secret *corev1.Secret) (*corev1.Secret, error) {
				return consumerKubeClient.CoreV1().Secrets(secret.Namespace).Update(ctx, secret, metav1.UpdateOptions{})
			},
		},

		commit: committer.NewCommitter[*kubebindv1alpha1.ClusterBinding, *kubebindv1alpha1.ClusterBindingSpec, *kubebindv1alpha1.ClusterBindingStatus](
			func(ns string) committer.Patcher[*kubebindv1alpha1.ClusterBinding] {
				return providerBindClient.KubeBindV1alpha1().ClusterBindings(ns)
			},
		),
	}

	if _, err := clusterBindingInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			c.enqueueClusterBinding(logger, obj)
		},
		UpdateFunc: func(_, newObj interface{}) {
			c.enqueueClusterBinding(logger, newObj)
		},
		DeleteFunc: func(obj interface{}) {
			c.enqueueClusterBinding(logger, obj)
		},
	}); err != nil {
		return nil, err
	}

	if _, err := providerSecretInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			c.enqueueProviderSecret(logger, obj)
		},
		UpdateFunc: func(_, newObj interface{}) {
			c.enqueueProviderSecret(logger, newObj)
		},
		DeleteFunc: func(obj interface{}) {
			c.enqueueProviderSecret(logger, obj)
		},
	}); err != nil {
		return nil, err
	}

	if _, err := consumerSecretInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			c.enqueueConsumerSecret(logger, obj)
		},
		UpdateFunc: func(_, newObj interface{}) {
			c.enqueueConsumerSecret(logger, newObj)
		},
		DeleteFunc: func(obj interface{}) {
			c.enqueueConsumerSecret(logger, obj)
		},
	}); err != nil {
		return nil, err
	}

	if _, err := serviceExportInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			c.enqueueServiceExport(logger, obj)
		},
		UpdateFunc: func(old, newObj interface{}) {
			oldExport, ok := old.(*kubebindv1alpha1.APIServiceExport)
			if !ok {
				return
			}
			newExport, ok := old.(*kubebindv1alpha1.APIServiceExport)
			if !ok {
				return
			}
			if reflect.DeepEqual(oldExport.Status.Conditions, newExport.Status.Conditions) {
				return
			}
			c.enqueueServiceExport(logger, newObj)
		},
		DeleteFunc: func(obj interface{}) {
			c.enqueueServiceExport(logger, obj)
		},
	}); err != nil {
		return nil, err
	}

	return c, nil
}

type Resource = committer.Resource[*kubebindv1alpha1.ClusterBindingSpec, *kubebindv1alpha1.ClusterBindingStatus]
type CommitFunc = func(context.Context, *Resource, *Resource) error

// controller reconciles ClusterBindings on the service provider cluster, including heartbeating.
type controller struct {
	queue workqueue.TypedRateLimitingInterface[string]

	providerBindClient bindclient.Interface
	providerKubeClient kubernetesclient.Interface
	consumerBindClient bindclient.Interface
	consumerKubeClient kubernetesclient.Interface

	clusterBindingLister  bindlisters.ClusterBindingLister
	clusterBindingIndexer cache.Indexer

	serviceBindingInformer dynamic.Informer[bindlisters.APIServiceBindingLister]

	serviceExportLister  bindlisters.APIServiceExportLister
	serviceExportIndexer cache.Indexer

	consumerSecretLister corelisters.SecretLister
	providerSecretLister corelisters.SecretLister

	reconciler

	commit CommitFunc
}

func (c *controller) enqueueClusterBinding(logger klog.Logger, obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		runtime.HandleError(err)
		return
	}

	logger.V(2).Info("queueing ClusterBinding", "key", key)
	c.queue.Add(key)
}

func (c *controller) enqueueConsumerSecret(logger klog.Logger, obj interface{}) {
	secretKey, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		runtime.HandleError(err)
		return
	}

	if secretKey == c.consumerSecretRefKey {
		key := c.providerNamespace + "/cluster"
		logger.V(2).Info("queueing ClusterBinding", "key", key, "reason", "ConsumerSecret", "SecretKey", secretKey)
		c.queue.Add(key)
	}
}

func (c *controller) enqueueProviderSecret(logger klog.Logger, obj interface{}) {
	secretKey, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		runtime.HandleError(err)
		return
	}
	ns, name, err := cache.SplitMetaNamespaceKey(secretKey)
	if err != nil {
		runtime.HandleError(err)
		return
	}

	if ns != c.providerNamespace {
		return // not our secret
	}

	binding, err := c.clusterBindingLister.ClusterBindings(ns).Get("cluster")
	if err != nil && !errors.IsNotFound(err) {
		runtime.HandleError(err)
		return
	} else if errors.IsNotFound(err) {
		return // skip this secret
	}
	if binding.Spec.KubeconfigSecretRef.Name != name {
		return // skip this secret
	}

	key := ns + "/cluster"
	logger.V(2).Info("queueing ClusterBinding", "key", key, "reason", "Secret", "SecretKey", secretKey)
	c.queue.Add(key)
}

func (c *controller) enqueueServiceExport(logger klog.Logger, obj interface{}) {
	seKey, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		runtime.HandleError(err)
		return
	}
	ns, _, err := cache.SplitMetaNamespaceKey(seKey)
	if err != nil {
		runtime.HandleError(err)
		return
	}

	key := ns + "/cluster"
	logger.V(2).Info("queueing ClusterBinding", "key", key, "reason", "APIServiceExport", "ServiceExportKey", seKey)
	c.queue.Add(key)
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

	// start the heartbeat
	//nolint:errcheck
	wait.PollUntilContextCancel(ctx, c.heartbeatInterval/2, false, func(ctx context.Context) (bool, error) {
		c.queue.Add(c.providerNamespace + "/cluster")
		return false, nil
	})

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
	if name != "cluster" {
		return nil // cannot happen by OpenAPI validation
	}

	logger := klog.FromContext(ctx)

	obj, err := c.clusterBindingLister.ClusterBindings(ns).Get("cluster")
	if err != nil && !errors.IsNotFound(err) {
		return err
	} else if errors.IsNotFound(err) {
		logger.Error(err, "ClusterBinding disappeared")
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

		// try to update service bindings
		c.updateServiceBindings(ctx, func(binding *kubebindv1alpha1.APIServiceBinding) {
			conditions.MarkFalse(
				binding,
				kubebindv1alpha1.APIServiceBindingConditionHeartbeating,
				"ClusterBindingUpdateFailed",
				conditionsapi.ConditionSeverityWarning,
				"Failed to update service provider ClusterBinding: %v", err,
			)
		})
	} else {
		// try to update service bindings
		c.updateServiceBindings(ctx, func(binding *kubebindv1alpha1.APIServiceBinding) {
			conditions.MarkTrue(binding, kubebindv1alpha1.APIServiceBindingConditionHeartbeating)
		})
	}

	return utilerrors.NewAggregate(errs)
}

func (c *controller) updateServiceBindings(ctx context.Context, update func(*kubebindv1alpha1.APIServiceBinding)) {
	logger := klog.FromContext(ctx)

	objs, err := c.serviceBindingInformer.Informer().GetIndexer().ByIndex(indexers.ByServiceBindingKubeconfigSecret, c.consumerSecretRefKey)
	if err != nil {
		logger.Error(err, "failed to list service bindings", "secretKey", c.consumerSecretRefKey)
		return
	}
	for _, obj := range objs {
		binding := obj.(*kubebindv1alpha1.APIServiceBinding)
		orig := binding
		binding = binding.DeepCopy()
		update(binding)
		if !reflect.DeepEqual(binding.Status.Conditions, orig.Status.Conditions) {
			logger.V(2).Info("updating service binding", "binding", binding.Name)
			if _, err := c.consumerBindClient.KubeBindV1alpha1().APIServiceBindings().UpdateStatus(ctx, binding, metav1.UpdateOptions{}); err != nil {
				logger.Error(err, "failed to update service binding", "binding", binding.Name)
				continue
			}
		}
	}
}
