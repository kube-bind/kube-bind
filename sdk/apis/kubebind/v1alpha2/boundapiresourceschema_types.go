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
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	conditionsapi "github.com/kube-bind/kube-bind/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
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

const (
	// BoundAPIResourceSchemaReady indicates that the API resource schema is ready.
	// It is set to true when the API resource schema is accepted and there are no drifts detected.
	BoundAPIResourceSchemaValid conditionsapi.ConditionType = "Valid"
	// BoundAPIResourceSchemaDriftDetected indicates that there is a drift between the consumer's API and the expected API.
	// It is set to true when the API resource schema is not accepted or there are drifts detected.
	BoundAPIResourceSchemaInvalid conditionsapi.ConditionType = "Invalid"
)

// BoundAPIResourceSchemaConditionReason is the set of reasons for specific condition type.
// +kubebuilder:validation:Enum=Accepted;Rejected;Pending;DriftDetected
type BoundAPIResourceSchemaConditionReason string

const (
	// BoundAPIResourceSchemaAccepted indicates that the API resource schema is accepted.
	BoundAPIResourceSchemaAccepted BoundAPIResourceSchemaConditionReason = "Accepted"
	// BoundAPIResourceSchemaRejected indicates that the API resource schema is rejected.
	BoundAPIResourceSchemaRejected BoundAPIResourceSchemaConditionReason = "Rejected"
	// BoundAPIResourceSchemaPending indicates that the API resource schema is pending.
	BoundAPIResourceSchemaPending BoundAPIResourceSchemaConditionReason = "Pending"
	// BoundAPIResourceSchemaDriftDetected indicates that there is a drift between the consumer's API and the expected API.
	BoundAPIResourceSchemaDriftDetected BoundAPIResourceSchemaConditionReason = "DriftDetected"
)

func (in *BoundAPIResourceSchema) GetConditions() conditionsapi.Conditions {
	return in.Status.Conditions
}

func (in *BoundAPIResourceSchema) SetConditions(conditions conditionsapi.Conditions) {
	in.Status.Conditions = conditions
}

// BoundAPIResourceSchemaStatus defines the observed state of the BoundAPIResourceSchema.
type BoundAPIResourceSchemaStatus struct {
	// acceptedNames are the names that are actually being used to serve discovery.
	// They may be different than the names in spec.
	// +optional
	AcceptedNames apiextensionsv1.CustomResourceDefinitionNames `json:"acceptedNames"`

	// storedVersions lists all versions of CustomResources that were ever persisted. Tracking these
	// versions allows a migration path for stored versions in etcd. The field is mutable
	// so a migration controller can finish a migration to another version (ensuring
	// no old objects are left in storage), and then remove the rest of the
	// versions from this list.
	// Versions may not be removed from `spec.versions` while they exist in this list.
	// +optional
	StoredVersions []string `json:"storedVersions"`
	// Conditions represent the latest available observations of the object's state.
	// +optional
	Conditions []conditionsapi.Condition `json:"conditions,omitempty"`

	// Instantiations tracks the number of instances of the resource on the consumer side.
	// +optional
	Instantiations int `json:"instantiations,omitempty"`
}

// BoundAPIResourceSchemaList is a list of BoundAPIResourceSchemas.
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type BoundAPIResourceSchemaList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []BoundAPIResourceSchema `json:"items"`
}
