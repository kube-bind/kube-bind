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

// APIServiceBindingClusterLister can list APIServiceBindings across all workspaces, or scope down to a APIServiceBindingLister for one workspace.
// All objects returned here must be treated as read-only.
type APIServiceBindingClusterLister interface {
	// List lists all APIServiceBindings in the indexer.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*kubebindv1alpha2.APIServiceBinding, err error)
	// Cluster returns a lister that can list and get APIServiceBindings in one workspace.
	Cluster(clusterName logicalcluster.Name) APIServiceBindingLister
	APIServiceBindingClusterListerExpansion
}

type aPIServiceBindingClusterLister struct {
	indexer cache.Indexer
}

// NewAPIServiceBindingClusterLister returns a new APIServiceBindingClusterLister.
// We assume that the indexer:
// - is fed by a cross-workspace LIST+WATCH
// - uses kcpcache.MetaClusterNamespaceKeyFunc as the key function
// - has the kcpcache.ClusterIndex as an index
func NewAPIServiceBindingClusterLister(indexer cache.Indexer) *aPIServiceBindingClusterLister {
	return &aPIServiceBindingClusterLister{indexer: indexer}
}

// List lists all APIServiceBindings in the indexer across all workspaces.
func (s *aPIServiceBindingClusterLister) List(selector labels.Selector) (ret []*kubebindv1alpha2.APIServiceBinding, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*kubebindv1alpha2.APIServiceBinding))
	})
	return ret, err
}

// Cluster scopes the lister to one workspace, allowing users to list and get APIServiceBindings.
func (s *aPIServiceBindingClusterLister) Cluster(clusterName logicalcluster.Name) APIServiceBindingLister {
	return &aPIServiceBindingLister{indexer: s.indexer, clusterName: clusterName}
}

// APIServiceBindingLister can list all APIServiceBindings, or get one in particular.
// All objects returned here must be treated as read-only.
type APIServiceBindingLister interface {
	// List lists all APIServiceBindings in the workspace.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*kubebindv1alpha2.APIServiceBinding, err error)
	// Get retrieves the APIServiceBinding from the indexer for a given workspace and name.
	// Objects returned here must be treated as read-only.
	Get(name string) (*kubebindv1alpha2.APIServiceBinding, error)
	APIServiceBindingListerExpansion
}

// aPIServiceBindingLister can list all APIServiceBindings inside a workspace.
type aPIServiceBindingLister struct {
	indexer     cache.Indexer
	clusterName logicalcluster.Name
}

// List lists all APIServiceBindings in the indexer for a workspace.
func (s *aPIServiceBindingLister) List(selector labels.Selector) (ret []*kubebindv1alpha2.APIServiceBinding, err error) {
	err = kcpcache.ListAllByCluster(s.indexer, s.clusterName, selector, func(i interface{}) {
		ret = append(ret, i.(*kubebindv1alpha2.APIServiceBinding))
	})
	return ret, err
}

// Get retrieves the APIServiceBinding from the indexer for a given workspace and name.
func (s *aPIServiceBindingLister) Get(name string) (*kubebindv1alpha2.APIServiceBinding, error) {
	key := kcpcache.ToClusterAwareKey(s.clusterName.String(), "", name)
	obj, exists, err := s.indexer.GetByKey(key)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(kubebindv1alpha2.Resource("apiservicebindings"), name)
	}
	return obj.(*kubebindv1alpha2.APIServiceBinding), nil
}

// NewAPIServiceBindingLister returns a new APIServiceBindingLister.
// We assume that the indexer:
// - is fed by a workspace-scoped LIST+WATCH
// - uses cache.MetaNamespaceKeyFunc as the key function
func NewAPIServiceBindingLister(indexer cache.Indexer) *aPIServiceBindingScopedLister {
	return &aPIServiceBindingScopedLister{indexer: indexer}
}

// aPIServiceBindingScopedLister can list all APIServiceBindings inside a workspace.
type aPIServiceBindingScopedLister struct {
	indexer cache.Indexer
}

// List lists all APIServiceBindings in the indexer for a workspace.
func (s *aPIServiceBindingScopedLister) List(selector labels.Selector) (ret []*kubebindv1alpha2.APIServiceBinding, err error) {
	err = cache.ListAll(s.indexer, selector, func(i interface{}) {
		ret = append(ret, i.(*kubebindv1alpha2.APIServiceBinding))
	})
	return ret, err
}

// Get retrieves the APIServiceBinding from the indexer for a given workspace and name.
func (s *aPIServiceBindingScopedLister) Get(name string) (*kubebindv1alpha2.APIServiceBinding, error) {
	key := name
	obj, exists, err := s.indexer.GetByKey(key)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(kubebindv1alpha2.Resource("apiservicebindings"), name)
	}
	return obj.(*kubebindv1alpha2.APIServiceBinding), nil
}
