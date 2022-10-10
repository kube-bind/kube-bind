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

// Code generated by lister-gen. DO NOT EDIT.

package v1alpha1

import (
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"

	v1alpha1 "github.com/kube-bind/kube-bind/pkg/apis/kubebind/v1alpha1"
)

// ServiceExportResourceLister helps list ServiceExportResources.
// All objects returned here must be treated as read-only.
type ServiceExportResourceLister interface {
	// List lists all ServiceExportResources in the indexer.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1alpha1.ServiceExportResource, err error)
	// ServiceExportResources returns an object that can list and get ServiceExportResources.
	ServiceExportResources(namespace string) ServiceExportResourceNamespaceLister
	ServiceExportResourceListerExpansion
}

// serviceExportResourceLister implements the ServiceExportResourceLister interface.
type serviceExportResourceLister struct {
	indexer cache.Indexer
}

// NewServiceExportResourceLister returns a new ServiceExportResourceLister.
func NewServiceExportResourceLister(indexer cache.Indexer) ServiceExportResourceLister {
	return &serviceExportResourceLister{indexer: indexer}
}

// List lists all ServiceExportResources in the indexer.
func (s *serviceExportResourceLister) List(selector labels.Selector) (ret []*v1alpha1.ServiceExportResource, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.ServiceExportResource))
	})
	return ret, err
}

// ServiceExportResources returns an object that can list and get ServiceExportResources.
func (s *serviceExportResourceLister) ServiceExportResources(namespace string) ServiceExportResourceNamespaceLister {
	return serviceExportResourceNamespaceLister{indexer: s.indexer, namespace: namespace}
}

// ServiceExportResourceNamespaceLister helps list and get ServiceExportResources.
// All objects returned here must be treated as read-only.
type ServiceExportResourceNamespaceLister interface {
	// List lists all ServiceExportResources in the indexer for a given namespace.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1alpha1.ServiceExportResource, err error)
	// Get retrieves the ServiceExportResource from the indexer for a given namespace and name.
	// Objects returned here must be treated as read-only.
	Get(name string) (*v1alpha1.ServiceExportResource, error)
	ServiceExportResourceNamespaceListerExpansion
}

// serviceExportResourceNamespaceLister implements the ServiceExportResourceNamespaceLister
// interface.
type serviceExportResourceNamespaceLister struct {
	indexer   cache.Indexer
	namespace string
}

// List lists all ServiceExportResources in the indexer for a given namespace.
func (s serviceExportResourceNamespaceLister) List(selector labels.Selector) (ret []*v1alpha1.ServiceExportResource, err error) {
	err = cache.ListAllByNamespace(s.indexer, s.namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.ServiceExportResource))
	})
	return ret, err
}

// Get retrieves the ServiceExportResource from the indexer for a given namespace and name.
func (s serviceExportResourceNamespaceLister) Get(name string) (*v1alpha1.ServiceExportResource, error) {
	obj, exists, err := s.indexer.GetByKey(s.namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v1alpha1.Resource("serviceexportresource"), name)
	}
	return obj.(*v1alpha1.ServiceExportResource), nil
}
