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

package cluster

import (
	"context"
	"fmt"
	"reflect"
	"time"

	crdlisters "k8s.io/apiextensions-apiserver/pkg/client/listers/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	kubernetesinformers "k8s.io/client-go/informers"
	kubernetesclient "k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	kubebindv1alpha1 "github.com/kube-bind/kube-bind/pkg/apis/kubebind/v1alpha1"
	conditionsapi "github.com/kube-bind/kube-bind/pkg/apis/third_party/conditions/apis/conditions/v1alpha1"
	"github.com/kube-bind/kube-bind/pkg/apis/third_party/conditions/util/conditions"
	bindclient "github.com/kube-bind/kube-bind/pkg/client/clientset/versioned"
	bindinformers "github.com/kube-bind/kube-bind/pkg/client/informers/externalversions"
	bindlisters "github.com/kube-bind/kube-bind/pkg/client/listers/kubebind/v1alpha1"
	"github.com/kube-bind/kube-bind/pkg/indexers"
	"github.com/kube-bind/kube-bind/pkg/konnector/controllers/cluster/clusterbinding"
	"github.com/kube-bind/kube-bind/pkg/konnector/controllers/cluster/namespacedeletion"
	"github.com/kube-bind/kube-bind/pkg/konnector/controllers/cluster/servicebinding"
	"github.com/kube-bind/kube-bind/pkg/konnector/controllers/cluster/serviceexport"
	"github.com/kube-bind/kube-bind/pkg/konnector/controllers/dynamic"
)

const (
	controllerName = "kube-bind-konnector-cluster"

	heartbeatInterval = 5 * time.Minute // TODO: make configurable
)

// NewController returns a new controller handling one cluster connection.
func NewController(
	consumerSecretRefKey string,
	providerNamespace string,
	consumerConfig, providerConfig *rest.Config,
	namespaceInformer dynamic.Informer[corelisters.NamespaceLister],
	serviceBindingInformer dynamic.Informer[bindlisters.ServiceBindingLister],
	crdInformer dynamic.Informer[crdlisters.CustomResourceDefinitionLister],
) (*controller, error) {
	consumerConfig = rest.CopyConfig(consumerConfig)
	consumerConfig = rest.AddUserAgent(consumerConfig, controllerName)

	providerConfig = rest.CopyConfig(providerConfig)
	providerConfig = rest.AddUserAgent(providerConfig, controllerName)

	// create shared informer factories
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
	providerBindInformers := bindinformers.NewSharedInformerFactoryWithOptions(providerBindClient, time.Minute*30, bindinformers.WithNamespace(providerNamespace))
	providerKubeInformers := kubernetesinformers.NewSharedInformerFactoryWithOptions(providerKubeClient, time.Minute*30, kubernetesinformers.WithNamespace(providerNamespace))
	consumerSecretNS, consumeSecretName, err := cache.SplitMetaNamespaceKey(consumerSecretRefKey)
	if err != nil {
		return nil, err
	}
	consumerSecretInformers := kubernetesinformers.NewSharedInformerFactoryWithOptions(consumerKubeClient, time.Minute*30,
		kubernetesinformers.WithNamespace(consumerSecretNS),
		kubernetesinformers.WithTweakListOptions(func(options *metav1.ListOptions) {
			options.FieldSelector = fmt.Sprintf("metadata.name=%s", consumeSecretName)
		}),
	)

	// create controllers
	clusterbindingCtrl, err := clusterbinding.NewController(
		consumerSecretRefKey,
		providerNamespace,
		heartbeatInterval,
		consumerConfig,
		providerConfig,
		providerBindInformers.KubeBind().V1alpha1().ClusterBindings(),
		serviceBindingInformer,
		providerBindInformers.KubeBind().V1alpha1().ServiceExports(),
		consumerSecretInformers.Core().V1().Secrets(),
		providerKubeInformers.Core().V1().Secrets(),
	)
	if err != nil {
		return nil, err
	}
	namespacedeletionCtrl, err := namespacedeletion.NewController(
		providerConfig,
		providerBindInformers.KubeBind().V1alpha1().ServiceNamespaces(),
		namespaceInformer,
	)
	if err != nil {
		return nil, err
	}
	serviceexportCtrl, err := serviceexport.NewController(
		consumerSecretRefKey,
		providerNamespace,
		consumerConfig,
		providerConfig,
		providerBindInformers.KubeBind().V1alpha1().ServiceExports(),
		providerBindInformers.KubeBind().V1alpha1().ServiceExportResources(),
		serviceBindingInformer,
	)
	if err != nil {
		return nil, err
	}
	servicebindingCtrl, err := servicebinding.NewController(
		consumerSecretRefKey,
		providerNamespace,
		consumerConfig,
		providerConfig,
		serviceBindingInformer,
		providerBindInformers.KubeBind().V1alpha1().ServiceExports(),
		providerBindInformers.KubeBind().V1alpha1().ServiceExportResources(),
		crdInformer,
	)
	if err != nil {
		return nil, err
	}

	return &controller{
		consumerSecretRefKey: consumerSecretRefKey,

		bindClient: consumerBindClient,

		factories: []SharedInformerFactory{
			providerBindInformers,
			providerKubeInformers,
			consumerSecretInformers,
		},

		serviceBindingLister:  serviceBindingInformer.Lister(),
		serviceBindingIndexer: serviceBindingInformer.Informer().GetIndexer(),

		clusterbindingCtrl:    clusterbindingCtrl,
		namespacedeletionCtrl: namespacedeletionCtrl,
		serviceexportCtrl:     serviceexportCtrl,
		servicebindingCtrl:    servicebindingCtrl,
	}, nil
}

type GenericController interface {
	Start(ctx context.Context, numThreads int)
}

type SharedInformerFactory interface {
	Start(stopCh <-chan struct{})
	WaitForCacheSync(stopCh <-chan struct{}) map[reflect.Type]bool
}

// controller holding all controller that are per provider cluster.
type controller struct {
	consumerSecretRefKey string

	bindClient bindclient.Interface

	serviceBindingLister  bindlisters.ServiceBindingLister
	serviceBindingIndexer cache.Indexer

	factories []SharedInformerFactory

	clusterbindingCtrl    GenericController
	namespacedeletionCtrl GenericController
	serviceexportCtrl     GenericController
	servicebindingCtrl    GenericController
}

// Start starts the controller, which stops when ctx.Done() is closed.
func (c *controller) Start(ctx context.Context) {
	logger := klog.FromContext(ctx).WithValues("controller", controllerName, "secretKey", c.consumerSecretRefKey)
	ctx = klog.NewContext(ctx, logger)

	logger.V(2).Info("starting factories")
	for _, factory := range c.factories {
		factory.Start(ctx.Done())
	}

	if err := wait.PollImmediateInfiniteWithContext(ctx, heartbeatInterval, func(ctx context.Context) (bool, error) {
		waitCtx, cancel := context.WithDeadline(ctx, time.Now().Add(heartbeatInterval/2))
		defer cancel()

		logger.V(2).Info("waiting for cache sync")
		for _, factory := range c.factories {
			synced := factory.WaitForCacheSync(waitCtx.Done())
			logger.V(2).Info("cache sync", "synced", synced)
		}
		select {
		case <-ctx.Done():
			// timeout
			logger.Info("informers did not sync in time", "timeout", heartbeatInterval/2)
			c.updateServiceBindings(ctx, func(binding *kubebindv1alpha1.ServiceBinding) {
				conditions.MarkFalse(
					binding,
					kubebindv1alpha1.ServiceBindingConditionInformersSynced,
					"InformerSyncTimeout",
					conditionsapi.ConditionSeverityError,
					"Informers did not sync within %s",
					heartbeatInterval/2,
				)
			})

			return false, nil
		default:
			return true, nil
		}
	}); err != nil {
		runtime.HandleError(err)
		return
	}

	logger.V(2).Info("setting InformersSynced condition to true on service binding")
	c.updateServiceBindings(ctx, func(binding *kubebindv1alpha1.ServiceBinding) {
		conditions.MarkTrue(binding, kubebindv1alpha1.ServiceBindingConditionInformersSynced)
	})

	go c.clusterbindingCtrl.Start(ctx, 2)
	go c.namespacedeletionCtrl.Start(ctx, 2)
	go c.serviceexportCtrl.Start(ctx, 2)
	go c.servicebindingCtrl.Start(ctx, 2)

	<-ctx.Done()
}

func (c *controller) updateServiceBindings(ctx context.Context, update func(*kubebindv1alpha1.ServiceBinding)) {
	logger := klog.FromContext(ctx)

	objs, err := c.serviceBindingIndexer.ByIndex(indexers.ByKubeconfigSecret, c.consumerSecretRefKey)
	if err != nil {
		logger.Error(err, "failed to list service bindings", "secretKey", c.consumerSecretRefKey)
		return
	}
	for _, obj := range objs {
		binding := obj.(*kubebindv1alpha1.ServiceBinding)
		orig := binding.DeepCopy()
		update(binding)
		if !reflect.DeepEqual(binding.Status.Conditions, orig.Status.Conditions) {
			logger.V(2).Info("updating service binding", "binding", binding.Name)
			if _, err := c.bindClient.KubeBindV1alpha1().ServiceBindings().UpdateStatus(ctx, binding, metav1.UpdateOptions{}); err != nil {
				logger.Error(err, "failed to update service binding", "binding", binding.Name)
				continue
			}
		}
	}
}
