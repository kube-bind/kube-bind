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
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
)

type prefixedStrategy struct {
	// On a technical level, this is the same as the basic "none" behaviour, just
	// with additional name mutation; since we want to re-use the other code for
	// cluster-scoped objects *on the provider side*, we extend the none strategy.
	noneStrategy

	clusterNamespace string
}

// NewPrefixed returns an isolation strategy for cluster-scoped objects where each
// object on the provider cluster gets the name of the cluster namespace prepended
// to their name (i.e. turning "my-obj" into "kube-bind-abc123-my-obj"). This is
// effective and easy since no scoping changes need to be accounted for (i.e. the
// BoundSchema does not need to be adjusted), but could theorically cause problems
// for objects with very long names that do not have enough room for such a prefix.
func NewPrefixed(clusterNamespace string, clusterNamespaceUID string) Strategy {
	return &prefixedStrategy{
		noneStrategy: noneStrategy{
			clusterNamespace:    clusterNamespace,
			clusterNamespaceUID: clusterNamespaceUID,
		},
		clusterNamespace: clusterNamespace,
	}
}

func (s *prefixedStrategy) ToProviderKey(consumerKey types.NamespacedName) (*types.NamespacedName, error) {
	return &types.NamespacedName{
		Name: s.clusterNamespace + "-" + consumerKey.Name,
	}, nil
}

func (s *prefixedStrategy) EnsureProviderKey(_ context.Context, consumerKey types.NamespacedName) (*types.NamespacedName, error) {
	return s.ToProviderKey(consumerKey)
}

func (s *prefixedStrategy) ToConsumerKey(providerKey types.NamespacedName) (*types.NamespacedName, error) {
	prefix := s.clusterNamespace + "-"
	if !strings.HasPrefix(providerKey.Name, prefix) {
		return nil, nil
	}

	return &types.NamespacedName{
		Name: strings.TrimPrefix(providerKey.Name, prefix),
	}, nil
}

func (s *prefixedStrategy) MutateMetadataAndSpec(consumerObj *unstructured.Unstructured, providerKey types.NamespacedName) error {
	consumerObj.SetName(providerKey.Name)
	consumerObj.SetNamespace(providerKey.Namespace)

	return s.noneStrategy.MutateMetadataAndSpec(consumerObj, providerKey)
}

func (s *prefixedStrategy) MutateStatus(providerObj *unstructured.Unstructured, consumerKey types.NamespacedName) error {
	providerObj.SetName(consumerKey.Name)
	providerObj.SetNamespace(consumerKey.Namespace)

	return s.noneStrategy.MutateStatus(providerObj, consumerKey)
}
