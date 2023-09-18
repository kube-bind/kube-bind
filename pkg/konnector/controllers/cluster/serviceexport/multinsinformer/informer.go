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

package multinsinformer

import (
	"context"
	"fmt"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	dynamicclient "k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	kubebindv1alpha1 "github.com/kube-bind/kube-bind/pkg/apis/kubebind/v1alpha1"
	bindlisters "github.com/kube-bind/kube-bind/pkg/client/listers/kubebind/v1alpha1"
	"github.com/kube-bind/kube-bind/pkg/indexers"
	"github.com/kube-bind/kube-bind/pkg/konnector/controllers/dynamic"
)

const (
	controllerName = "dynamic-multi-namespace-informer"
)

type GetterInformer interface {
	Get(ns, name string) (runtime.Object, error)
	List(ns string) ([]runtime.Object, error)
	AddEventHandler(handler cache.ResourceEventHandler)

	Start(ctx context.Context)
	WaitForCacheSync(stopCh <-chan struct{}) map[schema.GroupVersionResource]bool
}

var _ GetterInformer = &DynamicMultiNamespaceInformer{}

// DynamicMultiNamespaceInformer is a dynamic informer that spans multiple namespaces
// by starting individual informers per namespace and aggregating all of these.
type DynamicMultiNamespaceInformer struct {
	gvr               schema.GroupVersionResource
	providerNamespace string

	providerDynamicClient dynamicclient.Interface

	serviceNamespaceInformer dynamic.Informer[bindlisters.APIServiceNamespaceLister]

	lock               sync.RWMutex
	namespaceInformers map[string]informers.GenericInformer
	namespaceCancel    map[string]func()
	handlers           []cache.ResourceEventHandler
}

func NewDynamicMultiNamespaceInformer(
	gvr schema.GroupVersionResource,
	providerNamespace string,
	providerConfig *rest.Config,
	serviceNamespaceInformer dynamic.Informer[bindlisters.APIServiceNamespaceLister],
) (*DynamicMultiNamespaceInformer, error) {
	providerConfig = rest.CopyConfig(providerConfig)
	providerConfig = rest.AddUserAgent(providerConfig, controllerName)

	providerDynamicClient, err := dynamicclient.NewForConfig(providerConfig)
	if err != nil {
		return nil, err
	}

	inf := DynamicMultiNamespaceInformer{
		gvr:                      gvr,
		providerNamespace:        providerNamespace,
		providerDynamicClient:    providerDynamicClient,
		serviceNamespaceInformer: serviceNamespaceInformer,

		namespaceInformers: map[string]informers.GenericInformer{},
		namespaceCancel:    map[string]func(){},
	}

	return &inf, nil
}

func (inf *DynamicMultiNamespaceInformer) Start(ctx context.Context) {
	defer utilruntime.HandleCrash()

	logger := klog.FromContext(ctx).WithValues("controller", controllerName, "gvk", inf.gvr)
	ctx = klog.NewContext(ctx, logger)

	logger.Info("Starting controller")
	defer logger.Info("Shutting down controller")

	inf.serviceNamespaceInformer.Informer().AddDynamicEventHandler(ctx, controllerName, cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			inf.enqueueServiceNamespace(obj)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			inf.enqueueServiceNamespace(newObj)
		},
		DeleteFunc: func(obj interface{}) {
			inf.enqueueServiceNamespace(obj)
		},
	})

	go func() {
		<-ctx.Done()

		inf.lock.Lock()
		defer inf.lock.Unlock()
		for _, cancel := range inf.namespaceCancel {
			cancel()
		}
	}()
}

func (inf *DynamicMultiNamespaceInformer) enqueueServiceNamespace(obj interface{}) {
	logger := klog.FromContext(context.Background()).WithValues("controller", controllerName, "gvr", inf.gvr)

	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		utilruntime.HandleError(err)
		return
	}

	logger = logger.WithValues("key", key)
	logger.V(1).Info("processing APIServiceNamespace")

	ns, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		utilruntime.HandleError(err)
		return
	}
	if ns != inf.providerNamespace {
		return
	}

	sns, err := inf.serviceNamespaceInformer.Lister().APIServiceNamespaces(ns).Get(name)
	if err != nil && !errors.IsNotFound(err) {
		utilruntime.HandleError(err)
		return
	}
	if errors.IsNotFound(err) || sns.Status.Namespace == "" {
		if sns == nil {
			logger.V(2).Info("APIServiceNamespace disappeared")
		} else {
			logger.V(2).Info("APIServiceNamespace disappeared", "namespace", sns.Status.Namespace)
		}

		inf.lock.Lock()
		defer inf.lock.Unlock()
		if cancel, found := inf.namespaceCancel[name]; found {
			logger.V(2).Info("stopping informer", "namespace", sns.Status.Namespace)
			delete(inf.namespaceCancel, name)
			delete(inf.namespaceInformers, name)
			cancel()
		}
		return
	}

	inf.lock.Lock()
	defer inf.lock.Unlock()
	if _, found := inf.namespaceCancel[name]; found {
		logger.V(2).Info("informer for APIServiceNamespace already started", "namespace", sns.Status.Namespace)
		return // nothing to do, already running
	}

	logger.V(1).Info("starting dynamic informer", "namespace", sns.Status.Namespace)
	ctx, cancel := context.WithCancel(context.Background())
	factory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(inf.providerDynamicClient, time.Minute*30, sns.Status.Namespace, nil)
	gvrInf := factory.ForResource(inf.gvr)
	gvrInf.Lister() // to wire the GVR up in the informer factory
	inf.namespaceCancel[name] = cancel
	inf.namespaceInformers[name] = gvrInf

	for _, h := range inf.handlers {
		gvrInf.Informer().AddEventHandler(h)
	}

	factory.Start(ctx.Done())
}

func (inf *DynamicMultiNamespaceInformer) AddEventHandler(handler cache.ResourceEventHandler) {
	inf.lock.Lock()
	defer inf.lock.Unlock()

	for _, i := range inf.namespaceInformers {
		i.Informer().AddEventHandler(handler)
	}
	inf.handlers = append(inf.handlers, handler)
}

func (inf *DynamicMultiNamespaceInformer) Get(ns, name string) (runtime.Object, error) {
	snss, err := inf.serviceNamespaceInformer.Informer().GetIndexer().ByIndex(indexers.ServiceNamespaceByNamespace, ns)
	if err != nil {
		return nil, err
	}
	if len(snss) == 0 {
		return nil, errors.NewNotFound(inf.gvr.GroupResource(), name)
	} else if len(snss) > 1 {
		return nil, fmt.Errorf("unexpected multiple APIServiceNamespaces for namespace %s", ns)
	}
	sns := snss[0].(*kubebindv1alpha1.APIServiceNamespace)

	inf.lock.RLock()
	defer inf.lock.RUnlock()
	i, found := inf.namespaceInformers[sns.Name]
	if !found {
		return nil, errors.NewNotFound(inf.gvr.GroupResource(), name)
	}
	return i.Lister().ByNamespace(ns).Get(name)
}

func (inf *DynamicMultiNamespaceInformer) List(ns string) ([]runtime.Object, error) {
	snss, err := inf.serviceNamespaceInformer.Informer().GetIndexer().ByIndex(indexers.ServiceNamespaceByNamespace, ns)
	if err != nil {
		return nil, err
	}
	if len(snss) == 0 {
		return []runtime.Object{}, nil
	} else if len(snss) > 1 {
		return nil, fmt.Errorf("unexpected multiple APIServiceNamespaces for namespace %s", ns)
	}
	sns := snss[0].(*kubebindv1alpha1.APIServiceNamespace)

	inf.lock.RLock()
	defer inf.lock.RUnlock()
	i, found := inf.namespaceInformers[sns.Name]
	if !found {
		return []runtime.Object{}, nil
	}
	return i.Lister().ByNamespace(ns).List(labels.Everything())
}

func (inf *DynamicMultiNamespaceInformer) WaitForCacheSync(stopCh <-chan struct{}) map[schema.GroupVersionResource]bool {
	snss, err := inf.serviceNamespaceInformer.Lister().APIServiceNamespaces(inf.providerNamespace).List(labels.Everything())
	if err != nil {
		utilruntime.HandleError(err)
		return nil
	}

	if err := wait.PollImmediateUntil(time.Millisecond*100, func() (done bool, err error) {
		inf.lock.RLock()
		defer inf.lock.RUnlock()
		for _, sns := range snss {
			i, found := inf.namespaceInformers[sns.Name]
			if !found {
				// double check that the namespace is not gone
				if _, err := inf.serviceNamespaceInformer.Lister().APIServiceNamespaces(inf.providerNamespace).Get(sns.Name); err != nil {
					continue // ignore
				}
				return false, nil
			}
			if !i.Informer().HasSynced() {
				return false, nil
			}
		}
		return true, nil
	}, stopCh); err != nil {
		return map[schema.GroupVersionResource]bool{inf.gvr: false}
	}
	return map[schema.GroupVersionResource]bool{inf.gvr: true}
}

var _ GetterInformer = GetterInformerWrapper{}

type GetterInformerWrapper struct {
	GVR      schema.GroupVersionResource
	Delegate dynamicinformer.DynamicSharedInformerFactory
}

func (w GetterInformerWrapper) Get(ns, name string) (runtime.Object, error) {
	if ns != "" {
		return w.Delegate.ForResource(w.GVR).Lister().ByNamespace(ns).Get(name)
	}
	return w.Delegate.ForResource(w.GVR).Lister().Get(name)
}

func (w GetterInformerWrapper) List(ns string) ([]runtime.Object, error) {
	return w.Delegate.ForResource(w.GVR).Lister().ByNamespace(ns).List(labels.Everything())
}

func (w GetterInformerWrapper) AddEventHandler(handler cache.ResourceEventHandler) {
	w.Delegate.ForResource(w.GVR).Informer().AddEventHandler(handler)
}

func (w GetterInformerWrapper) Start(ctx context.Context) {
	w.Delegate.Start(ctx.Done())
}

func (w GetterInformerWrapper) WaitForCacheSync(stopCh <-chan struct{}) map[schema.GroupVersionResource]bool {
	return w.Delegate.WaitForCacheSync(stopCh)
}
