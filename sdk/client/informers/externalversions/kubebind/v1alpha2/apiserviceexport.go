/*
Copyright The Kube Bind Authors.

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

// Code generated by informer-gen. DO NOT EDIT.

package v1alpha2

import (
	"context"
	time "time"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	watch "k8s.io/apimachinery/pkg/watch"
	cache "k8s.io/client-go/tools/cache"

	kubebindv1alpha2 "github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2"
	versioned "github.com/kube-bind/kube-bind/sdk/client/clientset/versioned"
	internalinterfaces "github.com/kube-bind/kube-bind/sdk/client/informers/externalversions/internalinterfaces"
	v1alpha2 "github.com/kube-bind/kube-bind/sdk/client/listers/kubebind/v1alpha2"
)

// APIServiceExportInformer provides access to a shared informer and lister for
// APIServiceExports.
type APIServiceExportInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() v1alpha2.APIServiceExportLister
}

type aPIServiceExportInformer struct {
	factory          internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
	namespace        string
}

// NewAPIServiceExportInformer constructs a new informer for APIServiceExport type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewAPIServiceExportInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers) cache.SharedIndexInformer {
	return NewFilteredAPIServiceExportInformer(client, namespace, resyncPeriod, indexers, nil)
}

// NewFilteredAPIServiceExportInformer constructs a new informer for APIServiceExport type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewFilteredAPIServiceExportInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) cache.SharedIndexInformer {
	return cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options v1.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.KubeBindV1alpha2().APIServiceExports(namespace).List(context.TODO(), options)
			},
			WatchFunc: func(options v1.ListOptions) (watch.Interface, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.KubeBindV1alpha2().APIServiceExports(namespace).Watch(context.TODO(), options)
			},
		},
		&kubebindv1alpha2.APIServiceExport{},
		resyncPeriod,
		indexers,
	)
}

func (f *aPIServiceExportInformer) defaultInformer(client versioned.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
	return NewFilteredAPIServiceExportInformer(client, f.namespace, resyncPeriod, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, f.tweakListOptions)
}

func (f *aPIServiceExportInformer) Informer() cache.SharedIndexInformer {
	return f.factory.InformerFor(&kubebindv1alpha2.APIServiceExport{}, f.defaultInformer)
}

func (f *aPIServiceExportInformer) Lister() v1alpha2.APIServiceExportLister {
	return v1alpha2.NewAPIServiceExportLister(f.Informer().GetIndexer())
}
