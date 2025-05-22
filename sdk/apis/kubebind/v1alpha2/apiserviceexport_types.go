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

const (
	SourceSpecHashAnnotationKey = "kube-bind.io/source-spec-hash"
)

const (
	// APIServiceExportConditionConnected means the APIServiceExport has been connected to a APIServiceBinding.
	APIServiceExportConditionConnected conditionsapi.ConditionType = "Connected"

	// APIServiceExportConditionProviderInSync is set to true when the APIServiceExport
	// is in-sync with the CRD in the service provider cluster.
	APIServiceExportConditionProviderInSync conditionsapi.ConditionType = "ProviderInSync"

	// APIServiceExportConditionConsumerInSync is set to true when the APIServiceExport's
	// schema is applied to the consumer cluster.
	APIServiceExportConditionConsumerInSync conditionsapi.ConditionType = "ConsumerInSync"
)

// APIServiceExport specifies the resource to be exported. It is mostly a CRD:
// - the spec is a CRD spec, but without webhooks
// - the status reflects that on the consumer cluster
//
// +crd
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:scope=Namespaced,categories=kube-bindings
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
// +kubebuilder:printcolumn:name="Established",type="string",JSONPath=`.status.conditions[?(@.type=="Established")].status`,priority=5
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=`.metadata.creationTimestamp`,priority=0
// +kubebuilder:validation:XValidation:rule="self.metadata.name == self.spec.names.plural+\".\"+self.spec.group",message="informerScope is immutable"
type APIServiceExport struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec specifies the resource.
	// +required
	// +kubebuilder:validation:Required
	Spec APIServiceExportSpec `json:"spec"`

	// status contains reconciliation information for the resource.
	Status APIServiceExportStatus `json:"status,omitempty"`
}

func (in *APIServiceExport) GetConditions() conditionsapi.Conditions {
	return in.Status.Conditions
}

func (in *APIServiceExport) SetConditions(conditions conditionsapi.Conditions) {
	in.Status.Conditions = conditions
}

// APIServiceExportSpec defines the desired state of APIServiceExport.
//
// +kubebuilder:validation:XValidation:rule=`self.scope == "Namespaced" || self.informerScope == "Cluster"`,message="informerScope must be Cluster for cluster-scoped resources"
// +kubebuilder:validation:XValidation:rule=`self.scope == "Namespaced" || has(self.clusterScopedIsolation)`,message="clusterScopedIsolation must be defined for cluster-scoped resources"
// +kubebuilder:validation:XValidation:rule=`self.scope == "Cluster" || !has(self.clusterScopedIsolation)`,message="clusterScopedIsolation is not relevant for namespaced resources"
type APIServiceExportSpec struct {
	APIServiceExportCRDSpec APIResourceSchemaCRDSpec `json:",inline"`
	// resources specifies the API resources to export
	// +required
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	Resources []APIResourceSchemaReference `json:"resources"`
	// informerScope is the scope of the APIServiceExport. It can be either Cluster or Namespace.
	//
	// Cluster:    The konnector has permission to watch all namespaces at once and cluster-scoped resources.
	//             This is more efficient than watching each namespace individually.
	// Namespaced: The konnector has permission to watch only single namespaces.
	//             This is more resource intensive. And it means cluster-scoped resources cannot be exported.
	//
	// +required
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="informerScope is immutable"
	InformerScope InformerScope `json:"informerScope"`

	// ClusterScopedIsolation specifies how cluster scoped service objects are isolated between multiple consumers on the provider side.
	// It can be "Prefixed", "Namespaced", or "None".
	ClusterScopedIsolation Isolation `json:"clusterScopedIsolation,omitempty"`
}

// APIResourceSchemaReference is a list of references to APIResourceSchemas.
type APIResourceSchemaReference struct {
	// Name is the name of the resource to export
	// +required
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Type of the resource to export
	// Currently only APIResourceSchema is supported
	// +kubebuilder:validation:Enum=APIResourceSchema
	// +required
	Type string `json:"type"`
}

// Isolation is an enum defining the different ways to isolate cluster scoped objects
//
// +kubebuilder:validation:Enum=Prefixed;Namespaced;None
type Isolation string

const (
	// Prepends the name of the cluster namespace to an object's name.
	IsolationPrefixed Isolation = "Prefixed"

	// Maps a consumer side object into a namespaced object inside the corresponding cluster namespace.
	IsolationNamespaced Isolation = "Namespaced"

	// Used for the case of a dedicated provider where isolation is not necessary.
	IsolationNone Isolation = "None"
)

// APIServiceExportStatus stores status information about a APIServiceExport. It
// reflects the status of the CRD of the consumer cluster.
type APIServiceExportStatus struct {
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

	// conditions is a list of conditions that apply to the APIServiceExport. It is
	// updated by the konnector on the consumer cluster.
	Conditions conditionsapi.Conditions `json:"conditions,omitempty"`

	// boundSchemas contains references to all BoundAPIResourceSchema objects
	// associated with this APIServiceExport, tracking consumer usage status.
	// +optional
	// +listType=map
	// +listMapKey=name
	// +kubebuilder:validation:MinItems=1
	BoundSchemas []BoundSchemaReference `json:"boundSchemas,omitempty"`
}

// BoundSchemaReference contains a reference to a BoundAPIResourceSchema with status information.
type BoundSchemaReference struct {
	// name is the name of the BoundAPIResourceSchema.
	// +required
	Name string `json:"name"`

	// namespace is the namespace of the BoundAPIResourceSchema.
	// +required
	Namespace string `json:"namespace"`

	// Conditions represent the latest available observations of the object's state.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Instantiations tracks the number of instances of the resource on the consumer side.
	// +optional
	Instantiations int `json:"instantiations,omitempty"`
}

// APIServiceExportList is the objects list that represents the APIServiceExport.
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type APIServiceExportList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []APIServiceExport `json:"items"`
}
