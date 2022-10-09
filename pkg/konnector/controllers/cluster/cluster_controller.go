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

package cluster

import (
	"context"
	"fmt"
	"reflect"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubernetesinformers "k8s.io/client-go/informers"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/informers/internalinterfaces"
	kubernetesclient "k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	bindclient "github.com/kube-bind/kube-bind/pkg/client/clientset/versioned"
	bindinformers "github.com/kube-bind/kube-bind/pkg/client/informers/externalversions"
	bindv1alpha1informers "github.com/kube-bind/kube-bind/pkg/client/informers/externalversions/kubebind/v1alpha1"
	bindlisters "github.com/kube-bind/kube-bind/pkg/client/listers/kubebind/v1alpha1"
	"github.com/kube-bind/kube-bind/pkg/konnector/controllers/cluster/clusterbinding"
	"github.com/kube-bind/kube-bind/pkg/konnector/controllers/cluster/namespacedeletion"
)

const (
	controllerName = "kube-bind-konnector-cluster"
)

// NewController returns a new controller handling one cluster connection.
func NewController(
	consumerSecretRefKey string,
	providerNamespace string,
	consumerConfig, providerConfig *rest.Config,
	namespaceInformer coreinformers.NamespaceInformer,
	namespaceLister corelisters.NamespaceLister,
	serviceBindingsInformer bindv1alpha1informers.ServiceBindingInformer,
	serviceBidningsLister bindlisters.ServiceBindingLister, // intentional lister and informer here to protect against race
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
		time.Minute*10, // TODO: make configurable
		consumerConfig,
		providerConfig,
		providerBindInformers.KubeBind().V1alpha1().ClusterBindings(),
		serviceBindingsInformer,
		serviceBidningsLister,
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
		namespaceLister,
	)
	if err != nil {
		return nil, err
	}

	return &controller{
		consumerSecretRefKey: consumerSecretRefKey,

		clusterbindingCtrl:    clusterbindingCtrl,
		namespacedeletionCtrl: namespacedeletionCtrl,
	}, nil
}

type GenericController interface {
	Start(ctx context.Context, numThreads int)
}

type SharedInformerFactory interface {
	internalinterfaces.SharedInformerFactory
	WaitForCacheSync(stopCh <-chan struct{}) map[reflect.Type]bool
}

// controller holding all controller that are per provider cluster.
type controller struct {
	consumerSecretRefKey string

	factories []SharedInformerFactory

	clusterbindingCtrl    GenericController
	namespacedeletionCtrl GenericController
}

// Start starts the controller, which stops when ctx.Done() is closed.
func (c *controller) Start(ctx context.Context) {
	logger := klog.FromContext(ctx).WithValues("controller", controllerName, "secretKey", c.consumerSecretRefKey)
	ctx = klog.NewContext(ctx, logger)

	for _, factory := range c.factories {
		go factory.Start(ctx.Done())
	}
	for _, factory := range c.factories {
		factory.WaitForCacheSync(ctx.Done())
	}

	go c.clusterbindingCtrl.Start(ctx, 2)
	go c.namespacedeletionCtrl.Start(ctx, 2)

	<-ctx.Done()
}
