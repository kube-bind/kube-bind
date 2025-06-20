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

package v1alpha2

import (
	kcpcache "github.com/kcp-dev/apimachinery/v2/pkg/cache"
	"github.com/kcp-dev/logicalcluster/v3"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"

	kubebindv1alpha2 "github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2"
)

// APIServiceExportRequestClusterLister can list APIServiceExportRequests across all workspaces, or scope down to a APIServiceExportRequestLister for one workspace.
// All objects returned here must be treated as read-only.
type APIServiceExportRequestClusterLister interface {
	// List lists all APIServiceExportRequests in the indexer.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*kubebindv1alpha2.APIServiceExportRequest, err error)
	// Cluster returns a lister that can list and get APIServiceExportRequests in one workspace.
	Cluster(clusterName logicalcluster.Name) APIServiceExportRequestLister
	APIServiceExportRequestClusterListerExpansion
}

type aPIServiceExportRequestClusterLister struct {
	indexer cache.Indexer
}

// NewAPIServiceExportRequestClusterLister returns a new APIServiceExportRequestClusterLister.
// We assume that the indexer:
// - is fed by a cross-workspace LIST+WATCH
// - uses kcpcache.MetaClusterNamespaceKeyFunc as the key function
// - has the kcpcache.ClusterIndex as an index
// - has the kcpcache.ClusterAndNamespaceIndex as an index
func NewAPIServiceExportRequestClusterLister(indexer cache.Indexer) *aPIServiceExportRequestClusterLister {
	return &aPIServiceExportRequestClusterLister{indexer: indexer}
}

// List lists all APIServiceExportRequests in the indexer across all workspaces.
func (s *aPIServiceExportRequestClusterLister) List(selector labels.Selector) (ret []*kubebindv1alpha2.APIServiceExportRequest, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*kubebindv1alpha2.APIServiceExportRequest))
	})
	return ret, err
}

// Cluster scopes the lister to one workspace, allowing users to list and get APIServiceExportRequests.
func (s *aPIServiceExportRequestClusterLister) Cluster(clusterName logicalcluster.Name) APIServiceExportRequestLister {
	return &aPIServiceExportRequestLister{indexer: s.indexer, clusterName: clusterName}
}

// APIServiceExportRequestLister can list APIServiceExportRequests across all namespaces, or scope down to a APIServiceExportRequestNamespaceLister for one namespace.
// All objects returned here must be treated as read-only.
type APIServiceExportRequestLister interface {
	// List lists all APIServiceExportRequests in the workspace.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*kubebindv1alpha2.APIServiceExportRequest, err error)
	// APIServiceExportRequests returns a lister that can list and get APIServiceExportRequests in one workspace and namespace.
	APIServiceExportRequests(namespace string) APIServiceExportRequestNamespaceLister
	APIServiceExportRequestListerExpansion
}

// aPIServiceExportRequestLister can list all APIServiceExportRequests inside a workspace or scope down to a APIServiceExportRequestLister for one namespace.
type aPIServiceExportRequestLister struct {
	indexer     cache.Indexer
	clusterName logicalcluster.Name
}

// List lists all APIServiceExportRequests in the indexer for a workspace.
func (s *aPIServiceExportRequestLister) List(selector labels.Selector) (ret []*kubebindv1alpha2.APIServiceExportRequest, err error) {
	err = kcpcache.ListAllByCluster(s.indexer, s.clusterName, selector, func(i interface{}) {
		ret = append(ret, i.(*kubebindv1alpha2.APIServiceExportRequest))
	})
	return ret, err
}

// APIServiceExportRequests returns an object that can list and get APIServiceExportRequests in one namespace.
func (s *aPIServiceExportRequestLister) APIServiceExportRequests(namespace string) APIServiceExportRequestNamespaceLister {
	return &aPIServiceExportRequestNamespaceLister{indexer: s.indexer, clusterName: s.clusterName, namespace: namespace}
}

// aPIServiceExportRequestNamespaceLister helps list and get APIServiceExportRequests.
// All objects returned here must be treated as read-only.
type APIServiceExportRequestNamespaceLister interface {
	// List lists all APIServiceExportRequests in the workspace and namespace.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*kubebindv1alpha2.APIServiceExportRequest, err error)
	// Get retrieves the APIServiceExportRequest from the indexer for a given workspace, namespace and name.
	// Objects returned here must be treated as read-only.
	Get(name string) (*kubebindv1alpha2.APIServiceExportRequest, error)
	APIServiceExportRequestNamespaceListerExpansion
}

// aPIServiceExportRequestNamespaceLister helps list and get APIServiceExportRequests.
// All objects returned here must be treated as read-only.
type aPIServiceExportRequestNamespaceLister struct {
	indexer     cache.Indexer
	clusterName logicalcluster.Name
	namespace   string
}

// List lists all APIServiceExportRequests in the indexer for a given workspace and namespace.
func (s *aPIServiceExportRequestNamespaceLister) List(selector labels.Selector) (ret []*kubebindv1alpha2.APIServiceExportRequest, err error) {
	err = kcpcache.ListAllByClusterAndNamespace(s.indexer, s.clusterName, s.namespace, selector, func(i interface{}) {
		ret = append(ret, i.(*kubebindv1alpha2.APIServiceExportRequest))
	})
	return ret, err
}

// Get retrieves the APIServiceExportRequest from the indexer for a given workspace, namespace and name.
func (s *aPIServiceExportRequestNamespaceLister) Get(name string) (*kubebindv1alpha2.APIServiceExportRequest, error) {
	key := kcpcache.ToClusterAwareKey(s.clusterName.String(), s.namespace, name)
	obj, exists, err := s.indexer.GetByKey(key)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(kubebindv1alpha2.Resource("apiserviceexportrequests"), name)
	}
	return obj.(*kubebindv1alpha2.APIServiceExportRequest), nil
}

// NewAPIServiceExportRequestLister returns a new APIServiceExportRequestLister.
// We assume that the indexer:
// - is fed by a workspace-scoped LIST+WATCH
// - uses cache.MetaNamespaceKeyFunc as the key function
// - has the cache.NamespaceIndex as an index
func NewAPIServiceExportRequestLister(indexer cache.Indexer) *aPIServiceExportRequestScopedLister {
	return &aPIServiceExportRequestScopedLister{indexer: indexer}
}

// aPIServiceExportRequestScopedLister can list all APIServiceExportRequests inside a workspace or scope down to a APIServiceExportRequestLister for one namespace.
type aPIServiceExportRequestScopedLister struct {
	indexer cache.Indexer
}

// List lists all APIServiceExportRequests in the indexer for a workspace.
func (s *aPIServiceExportRequestScopedLister) List(selector labels.Selector) (ret []*kubebindv1alpha2.APIServiceExportRequest, err error) {
	err = cache.ListAll(s.indexer, selector, func(i interface{}) {
		ret = append(ret, i.(*kubebindv1alpha2.APIServiceExportRequest))
	})
	return ret, err
}

// APIServiceExportRequests returns an object that can list and get APIServiceExportRequests in one namespace.
func (s *aPIServiceExportRequestScopedLister) APIServiceExportRequests(namespace string) APIServiceExportRequestNamespaceLister {
	return &aPIServiceExportRequestScopedNamespaceLister{indexer: s.indexer, namespace: namespace}
}

// aPIServiceExportRequestScopedNamespaceLister helps list and get APIServiceExportRequests.
type aPIServiceExportRequestScopedNamespaceLister struct {
	indexer   cache.Indexer
	namespace string
}

// List lists all APIServiceExportRequests in the indexer for a given workspace and namespace.
func (s *aPIServiceExportRequestScopedNamespaceLister) List(selector labels.Selector) (ret []*kubebindv1alpha2.APIServiceExportRequest, err error) {
	err = cache.ListAllByNamespace(s.indexer, s.namespace, selector, func(i interface{}) {
		ret = append(ret, i.(*kubebindv1alpha2.APIServiceExportRequest))
	})
	return ret, err
}

// Get retrieves the APIServiceExportRequest from the indexer for a given workspace, namespace and name.
func (s *aPIServiceExportRequestScopedNamespaceLister) Get(name string) (*kubebindv1alpha2.APIServiceExportRequest, error) {
	key := s.namespace + "/" + name
	obj, exists, err := s.indexer.GetByKey(key)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(kubebindv1alpha2.Resource("apiserviceexportrequests"), name)
	}
	return obj.(*kubebindv1alpha2.APIServiceExportRequest), nil
}
