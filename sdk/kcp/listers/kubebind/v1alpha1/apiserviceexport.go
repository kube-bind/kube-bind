//go:build !ignore_autogenerated
// +build !ignore_autogenerated

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

// Code generated by kcp code-generator. DO NOT EDIT.

package v1alpha1

import (
	kcpcache "github.com/kcp-dev/apimachinery/v2/pkg/cache"
	"github.com/kcp-dev/logicalcluster/v3"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"

	kubebindv1alpha1 "github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha1"
)

// APIServiceExportClusterLister can list APIServiceExports across all workspaces, or scope down to a APIServiceExportLister for one workspace.
// All objects returned here must be treated as read-only.
type APIServiceExportClusterLister interface {
	// List lists all APIServiceExports in the indexer.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*kubebindv1alpha1.APIServiceExport, err error)
	// Cluster returns a lister that can list and get APIServiceExports in one workspace.
	Cluster(clusterName logicalcluster.Name) APIServiceExportLister
	APIServiceExportClusterListerExpansion
}

type aPIServiceExportClusterLister struct {
	indexer cache.Indexer
}

// NewAPIServiceExportClusterLister returns a new APIServiceExportClusterLister.
// We assume that the indexer:
// - is fed by a cross-workspace LIST+WATCH
// - uses kcpcache.MetaClusterNamespaceKeyFunc as the key function
// - has the kcpcache.ClusterIndex as an index
// - has the kcpcache.ClusterAndNamespaceIndex as an index
func NewAPIServiceExportClusterLister(indexer cache.Indexer) *aPIServiceExportClusterLister {
	return &aPIServiceExportClusterLister{indexer: indexer}
}

// List lists all APIServiceExports in the indexer across all workspaces.
func (s *aPIServiceExportClusterLister) List(selector labels.Selector) (ret []*kubebindv1alpha1.APIServiceExport, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*kubebindv1alpha1.APIServiceExport))
	})
	return ret, err
}

// Cluster scopes the lister to one workspace, allowing users to list and get APIServiceExports.
func (s *aPIServiceExportClusterLister) Cluster(clusterName logicalcluster.Name) APIServiceExportLister {
	return &aPIServiceExportLister{indexer: s.indexer, clusterName: clusterName}
}

// APIServiceExportLister can list APIServiceExports across all namespaces, or scope down to a APIServiceExportNamespaceLister for one namespace.
// All objects returned here must be treated as read-only.
type APIServiceExportLister interface {
	// List lists all APIServiceExports in the workspace.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*kubebindv1alpha1.APIServiceExport, err error)
	// APIServiceExports returns a lister that can list and get APIServiceExports in one workspace and namespace.
	APIServiceExports(namespace string) APIServiceExportNamespaceLister
	APIServiceExportListerExpansion
}

// aPIServiceExportLister can list all APIServiceExports inside a workspace or scope down to a APIServiceExportLister for one namespace.
type aPIServiceExportLister struct {
	indexer     cache.Indexer
	clusterName logicalcluster.Name
}

// List lists all APIServiceExports in the indexer for a workspace.
func (s *aPIServiceExportLister) List(selector labels.Selector) (ret []*kubebindv1alpha1.APIServiceExport, err error) {
	err = kcpcache.ListAllByCluster(s.indexer, s.clusterName, selector, func(i interface{}) {
		ret = append(ret, i.(*kubebindv1alpha1.APIServiceExport))
	})
	return ret, err
}

// APIServiceExports returns an object that can list and get APIServiceExports in one namespace.
func (s *aPIServiceExportLister) APIServiceExports(namespace string) APIServiceExportNamespaceLister {
	return &aPIServiceExportNamespaceLister{indexer: s.indexer, clusterName: s.clusterName, namespace: namespace}
}

// aPIServiceExportNamespaceLister helps list and get APIServiceExports.
// All objects returned here must be treated as read-only.
type APIServiceExportNamespaceLister interface {
	// List lists all APIServiceExports in the workspace and namespace.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*kubebindv1alpha1.APIServiceExport, err error)
	// Get retrieves the APIServiceExport from the indexer for a given workspace, namespace and name.
	// Objects returned here must be treated as read-only.
	Get(name string) (*kubebindv1alpha1.APIServiceExport, error)
	APIServiceExportNamespaceListerExpansion
}

// aPIServiceExportNamespaceLister helps list and get APIServiceExports.
// All objects returned here must be treated as read-only.
type aPIServiceExportNamespaceLister struct {
	indexer     cache.Indexer
	clusterName logicalcluster.Name
	namespace   string
}

// List lists all APIServiceExports in the indexer for a given workspace and namespace.
func (s *aPIServiceExportNamespaceLister) List(selector labels.Selector) (ret []*kubebindv1alpha1.APIServiceExport, err error) {
	err = kcpcache.ListAllByClusterAndNamespace(s.indexer, s.clusterName, s.namespace, selector, func(i interface{}) {
		ret = append(ret, i.(*kubebindv1alpha1.APIServiceExport))
	})
	return ret, err
}

// Get retrieves the APIServiceExport from the indexer for a given workspace, namespace and name.
func (s *aPIServiceExportNamespaceLister) Get(name string) (*kubebindv1alpha1.APIServiceExport, error) {
	key := kcpcache.ToClusterAwareKey(s.clusterName.String(), s.namespace, name)
	obj, exists, err := s.indexer.GetByKey(key)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(kubebindv1alpha1.Resource("apiserviceexports"), name)
	}
	return obj.(*kubebindv1alpha1.APIServiceExport), nil
}

// NewAPIServiceExportLister returns a new APIServiceExportLister.
// We assume that the indexer:
// - is fed by a workspace-scoped LIST+WATCH
// - uses cache.MetaNamespaceKeyFunc as the key function
// - has the cache.NamespaceIndex as an index
func NewAPIServiceExportLister(indexer cache.Indexer) *aPIServiceExportScopedLister {
	return &aPIServiceExportScopedLister{indexer: indexer}
}

// aPIServiceExportScopedLister can list all APIServiceExports inside a workspace or scope down to a APIServiceExportLister for one namespace.
type aPIServiceExportScopedLister struct {
	indexer cache.Indexer
}

// List lists all APIServiceExports in the indexer for a workspace.
func (s *aPIServiceExportScopedLister) List(selector labels.Selector) (ret []*kubebindv1alpha1.APIServiceExport, err error) {
	err = cache.ListAll(s.indexer, selector, func(i interface{}) {
		ret = append(ret, i.(*kubebindv1alpha1.APIServiceExport))
	})
	return ret, err
}

// APIServiceExports returns an object that can list and get APIServiceExports in one namespace.
func (s *aPIServiceExportScopedLister) APIServiceExports(namespace string) APIServiceExportNamespaceLister {
	return &aPIServiceExportScopedNamespaceLister{indexer: s.indexer, namespace: namespace}
}

// aPIServiceExportScopedNamespaceLister helps list and get APIServiceExports.
type aPIServiceExportScopedNamespaceLister struct {
	indexer   cache.Indexer
	namespace string
}

// List lists all APIServiceExports in the indexer for a given workspace and namespace.
func (s *aPIServiceExportScopedNamespaceLister) List(selector labels.Selector) (ret []*kubebindv1alpha1.APIServiceExport, err error) {
	err = cache.ListAllByNamespace(s.indexer, s.namespace, selector, func(i interface{}) {
		ret = append(ret, i.(*kubebindv1alpha1.APIServiceExport))
	})
	return ret, err
}

// Get retrieves the APIServiceExport from the indexer for a given workspace, namespace and name.
func (s *aPIServiceExportScopedNamespaceLister) Get(name string) (*kubebindv1alpha1.APIServiceExport, error) {
	key := s.namespace + "/" + name
	obj, exists, err := s.indexer.GetByKey(key)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(kubebindv1alpha1.Resource("apiserviceexports"), name)
	}
	return obj.(*kubebindv1alpha1.APIServiceExport), nil
}