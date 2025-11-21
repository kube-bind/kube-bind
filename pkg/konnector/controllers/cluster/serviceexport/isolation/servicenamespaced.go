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

package isolation

import (
	"context"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"

	"github.com/kube-bind/kube-bind/pkg/indexers"
	"github.com/kube-bind/kube-bind/pkg/konnector/controllers/dynamic"
	kubebindv1alpha2 "github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2"
	bindlisters "github.com/kube-bind/kube-bind/sdk/client/listers/kubebind/v1alpha2"
)

type ServiceNamespacedStrategy struct {
	clusterNamespace         string
	serviceNamespaceInformer dynamic.Informer[bindlisters.APIServiceNamespaceLister]
	serviceNamespaceCreator  func(ctx context.Context, name string) (*kubebindv1alpha2.APIServiceNamespace, error)
}

// NewServiceNamespaced returns the one and only valid isolation strategy for
// namespaced objects. It is special in a sense that it does not map consumer
// namespaces 1:1 to provider namspaces, but uses APIServiceNamespace objects
// to request the backend to assign a namespace on the provider cluster. This
// strategy must not be used for cluster-scoped resources.
func NewServiceNamespaced(
	clusterNamespace string,
	serviceNamespaceInformer dynamic.Informer[bindlisters.APIServiceNamespaceLister],
	serviceNamespaceCreator func(ctx context.Context, name string) (*kubebindv1alpha2.APIServiceNamespace, error),
) Strategy {
	return &ServiceNamespacedStrategy{
		clusterNamespace:         clusterNamespace,
		serviceNamespaceInformer: serviceNamespaceInformer,
		serviceNamespaceCreator:  serviceNamespaceCreator,
	}
}

func (s *ServiceNamespacedStrategy) ProviderNamespace(consumerNamespace string) (string, error) {
	sn, err := s.serviceNamespaceInformer.Lister().APIServiceNamespaces(s.clusterNamespace).Get(consumerNamespace)
	if err != nil {
		if !errors.IsNotFound(err) {
			return "", err
		}

		return "", nil
	}

	return sn.Status.Namespace, nil
}

// ConsumerNamespace returns the namespace on the consumer cluster that owns the
// API namespace on the provider cluster. This function effectively performs a
// reverse lookup using the APIServiceNamespace's status.
// This function can return an empty name if the APIServiceNamespace is not ready yet.
func (s *ServiceNamespacedStrategy) ConsumerNamespace(namespace string) (string, error) {
	sns, err := s.serviceNamespaceInformer.Informer().GetIndexer().ByIndex(indexers.ServiceNamespaceByNamespace, namespace)
	if err != nil {
		return "", err
	}

	for _, obj := range sns {
		sns := obj.(*kubebindv1alpha2.APIServiceNamespace)
		if sns.Namespace == s.clusterNamespace {
			return sns.Name, nil
		}
	}

	return "", nil
}

func (s *ServiceNamespacedStrategy) ToProviderKey(consumerKey types.NamespacedName) (*types.NamespacedName, error) {
	providerNs, err := s.ProviderNamespace(consumerKey.Namespace)
	if err != nil {
		return nil, err
	}

	// not ready yet
	if providerNs == "" {
		return nil, nil
	}

	return &types.NamespacedName{
		Namespace: providerNs,
		Name:      consumerKey.Name,
	}, nil
}

func (s *ServiceNamespacedStrategy) EnsureProviderKey(ctx context.Context, consumerKey types.NamespacedName) (*types.NamespacedName, error) {
	sn, err := s.serviceNamespaceInformer.Lister().APIServiceNamespaces(s.clusterNamespace).Get(consumerKey.Namespace)
	if err != nil && !errors.IsNotFound(err) {
		return nil, err
	}

	logger := klog.FromContext(ctx).WithValues("namespace", consumerKey.Namespace)

	if errors.IsNotFound(err) {
		logger.V(1).Info("creating APIServiceNamespace")
		sn, err = s.serviceNamespaceCreator(ctx, consumerKey.Namespace)
		if err != nil {
			return nil, err
		}
	}

	if sn.Status.Namespace == "" {
		// note: the service provider might implement this synchronously in admission. if so, we can skip the requeue.
		logger.V(1).Info("waiting for APIServiceNamespace to be ready")
		return nil, nil
	}

	return &types.NamespacedName{
		Namespace: sn.Status.Namespace,
		Name:      consumerKey.Name,
	}, nil
}

func (s *ServiceNamespacedStrategy) ToConsumerKey(providerKey types.NamespacedName) (*types.NamespacedName, error) {
	consumerNs, err := s.ConsumerNamespace(providerKey.Namespace)
	if err != nil {
		return nil, err
	}

	// not ready yet
	if consumerNs == "" {
		return nil, nil
	}

	return &types.NamespacedName{
		Namespace: consumerNs,
		Name:      providerKey.Name,
	}, nil
}

func (s *ServiceNamespacedStrategy) MutateMetadataAndSpec(consumerObj *unstructured.Unstructured, providerKey types.NamespacedName) error {
	consumerObj.SetName(providerKey.Name)
	consumerObj.SetNamespace(providerKey.Namespace)

	return nil
}

func (s *ServiceNamespacedStrategy) MutateStatus(providerObj *unstructured.Unstructured, consumerKey types.NamespacedName) error {
	providerObj.SetName(consumerKey.Name)
	providerObj.SetNamespace(consumerKey.Namespace)

	return nil
}
