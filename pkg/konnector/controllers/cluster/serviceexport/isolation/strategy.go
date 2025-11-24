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

// Strategy implements the namespace/name translation for Kubernetes objects when
// they are copied from one cluster to another. Not every strategy is necessarily
// applicable to all circumstances (strategies can be for cluster-scoped or
// namespaced objects only, check their documentation).
type Strategy interface {
	// ToProviderKey translates the namespace/name (collectively called "key") from
	// the consumer side to the provider side. This function can return a nil key
	// for invalid/foreign objects, so callers need to be aware.
	ToProviderKey(consumerKey types.NamespacedName) (*types.NamespacedName, error)

	// ToConsumerKey translates the namespace/name (collectively called "key") from
	// the provider side to the consumer side. This function can return a nil key
	// for invalid/foreign objects, so callers need to be aware.
	ToConsumerKey(providerKey types.NamespacedName) (*types.NamespacedName, error)

	// EnsureProviderKey is very similar to ToProviderKey, but may make changes
	// on the provider side and return a nil key in case the target key is simply
	// not ready yet. This is most often the case when waiting for the backend
	// to assign a namespace for an APIServiceNamespace.
	EnsureProviderKey(ctx context.Context, consumerKey types.NamespacedName) (*types.NamespacedName, error)

	// MutateMetadataAndSpec mutates the object's content (its main resource) when
	// syncing from the consumer side to the provide side. This function is also
	// responsible for applying the translated keys (from ToProviderKey).
	MutateMetadataAndSpec(consumerObj *unstructured.Unstructured, providerKey types.NamespacedName) error

	// MutateStatus is the opposite of MutateMetadataAndSpec and might mutate the
	// object status (status subresource) when syncing it back from the provider
	// cluster to the consumer cluster. This function is also responsible for
	// applying the consumer key to "rename" the object back to its original name
	// on the consumer cluster.
	MutateStatus(providerObj *unstructured.Unstructured, consumerKey types.NamespacedName) error
}
