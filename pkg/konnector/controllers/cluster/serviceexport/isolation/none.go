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
	"errors"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
)

type noneStrategy struct {
	clusterNamespace    string
	clusterNamespaceUID string
}

// NewNone returns an isolation strategy for cluster-scoped objects that will not
// change anything about their name. Using this strategy will most likely lead to
// naming conflicts between consumers and should only be used with great care.
// Since this strategy will lead to cluster-scoped objects on the provide side,
// it will add an owner reference to each, pointing to the cluster namespace.
func NewNone(clusterNamespace string, clusterNamespaceUID string) Strategy {
	return &noneStrategy{
		clusterNamespace:    clusterNamespace,
		clusterNamespaceUID: clusterNamespaceUID,
	}
}

func (*noneStrategy) ToProviderKey(consumerKey types.NamespacedName) (*types.NamespacedName, error) {
	return &consumerKey, nil
}

func (s *noneStrategy) EnsureProviderKey(_ context.Context, consumerKey types.NamespacedName) (*types.NamespacedName, error) {
	return s.ToProviderKey(consumerKey)
}

func (*noneStrategy) ToConsumerKey(providerKey types.NamespacedName) (*types.NamespacedName, error) {
	return &providerKey, nil
}

func (s *noneStrategy) MutateMetadataAndSpec(consumerObj *unstructured.Unstructured, providerKey types.NamespacedName) error {
	return setOwnerReference(consumerObj, s.clusterNamespace, s.clusterNamespaceUID)
}

func (*noneStrategy) MutateStatus(providerObj *unstructured.Unstructured, consumerKey types.NamespacedName) error {
	return nil
}

func setOwnerReference(obj *unstructured.Unstructured, clusterNamespace string, clusterNamespaceUID string) error {
	ownerRefs := obj.GetOwnerReferences()
	ownerRef := findOwnerReference(ownerRefs, clusterNamespace)
	if ownerRef != nil && ownerRef.Name != clusterNamespace {
		return errors.New("mismatch between existing cluster namespace and given cluster namespace")
	}

	if ownerRef == nil {
		ownerRefs = append(ownerRefs, metav1.OwnerReference{
			APIVersion: "v1",
			Kind:       "Namespace",
			Name:       clusterNamespace,
			UID:        types.UID(clusterNamespaceUID),
		})
		obj.SetOwnerReferences(ownerRefs)
	}

	return nil
}

func findOwnerReference(refs []metav1.OwnerReference, clusterNamespace string) *metav1.OwnerReference {
	for _, ref := range refs {
		if ref.Kind == "Namespace" && ref.Name == clusterNamespace {
			return &ref
		}
	}

	return nil
}
