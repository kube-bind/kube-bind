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

package types

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// ClusterNamespaceAnnotationKey is the annotation key to identify the
// cluster namespace that any synced object on the provider side belongs to.
// This annotation is set on all synced objects, regardless of scope or
// isolation mode.
const ClusterNamespaceAnnotationKey = "kube-bind.io/cluster-namespace"

// ConsumerNamespaceAnnotationKey is the annotation key set on provider-side
// objects to identify the consumer namespace the source object lives in.
const ConsumerNamespaceAnnotationKey = "kube-bind.io/consumer-namespace"

// ConsumerUIDAnnotationKey is the annotation key set on provider-side objects
// to uniquely identify the consumer source object.
const ConsumerUIDAnnotationKey = "kube-bind.io/consumer-uid"

// ProviderNamespaceAnnotationKey is the annotation key set on consumer-side
// objects to identify the provider namespace the source object lives in.
const ProviderNamespaceAnnotationKey = "kube-bind.io/provider-namespace"

// ProviderUIDAnnotationKey is the annotation key set on consumer-side objects
// to uniquely identify the provider source object.
const ProviderUIDAnnotationKey = "kube-bind.io/provider-uid"

// SetSourceMetadataAnnotations sets source metadata annotations on a synced
// object so the receiving side can trace where the object came from.
func SetSourceMetadataAnnotations(obj *unstructured.Unstructured, sourceNS, sourceUID, nsKey, uidKey string) {
	annotations := obj.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	annotations[nsKey] = sourceNS
	annotations[uidKey] = sourceUID
	obj.SetAnnotations(annotations)
}
