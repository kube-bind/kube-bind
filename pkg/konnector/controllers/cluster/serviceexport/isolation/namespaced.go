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

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
)

type namespacedStrategy struct {
	clusterNamespace string
}

// NewNamespaced returns an isolation strategy that transforms cluster-scoped
// objects from the consumer cluster into namespaced objects on the provider
// cluster. All objects will be placed in the cluster namespace.
func NewNamespaced(clusterNamespace string) Strategy {
	return &namespacedStrategy{
		clusterNamespace: clusterNamespace,
	}
}

func (s *namespacedStrategy) ToProviderKey(consumerKey types.NamespacedName) (*types.NamespacedName, error) {
	return &types.NamespacedName{
		Namespace: s.clusterNamespace,
		Name:      consumerKey.Name,
	}, nil
}

func (s *namespacedStrategy) EnsureProviderKey(_ context.Context, consumerKey types.NamespacedName) (*types.NamespacedName, error) {
	return s.ToProviderKey(consumerKey)
}

func (s *namespacedStrategy) ToConsumerKey(providerKey types.NamespacedName) (*types.NamespacedName, error) {
	if providerKey.Namespace != s.clusterNamespace {
		return nil, nil
	}

	return &types.NamespacedName{Name: providerKey.Name}, nil
}

func (s *namespacedStrategy) MutateMetadataAndSpec(consumerObj *unstructured.Unstructured, providerKey types.NamespacedName) error {
	consumerObj.SetName(providerKey.Name)
	consumerObj.SetNamespace(providerKey.Namespace)

	return nil
}

func (s *namespacedStrategy) MutateStatus(providerObj *unstructured.Unstructured, consumerKey types.NamespacedName) error {
	providerObj.SetName(consumerKey.Name)
	providerObj.SetNamespace(consumerKey.Namespace)

	return nil
}
