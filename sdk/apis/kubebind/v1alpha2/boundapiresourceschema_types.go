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

package v1alpha2

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// InformerScope is the scope of the Api.
//
// +kubebuilder:validation:Enum=Cluster;Namespaced
type InformerScope string

const (
	ClusterScope    InformerScope = "Cluster"
	NamespacedScope InformerScope = "Namespaced"
)

// BoundAPIResourceSchema
// +crd
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:scope=Namespaced,categories=kube-bindings
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type BoundAPIResourceSchema struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BoundAPIResourceSchemaSpec   `json:"spec"`
	Status BoundAPIResourceSchemaStatus `json:"status,omitempty"`
}

// BoundAPIResourceSchemaSpec defines the desired state of the BoundAPIResourceSchema.
type BoundAPIResourceSchemaSpec struct {
	// InformerScope indicates whether the informer for defined custom resource is cluster- or namespace-scoped.
	// Allowed values are `Cluster` and `Namespaced`.
	//
	// +required
	// +kubebuilder:validation:Enum=Cluster;Namespaced
	InformerScope InformerScope `json:"informerScope"`

	APIResourceSchemaCRDSpec `json:",inline"`
}

// BoundAPIResourceSchemaStatus defines the observed state of the BoundAPIResourceSchema.
type BoundAPIResourceSchemaStatus struct {
	// Conditions represent the latest available observations of the object's state.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// BoundAPIResourceSchemaList is a list of BoundAPIResourceSchemas.
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type BoundAPIResourceSchemaList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []BoundAPIResourceSchema `json:"items"`
}

type APIResourceSchemaCRDSpec struct {
}
