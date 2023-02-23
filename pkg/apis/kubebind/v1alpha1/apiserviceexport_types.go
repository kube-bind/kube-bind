/*
Copyright 2022 The Kube Bind Authors.

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

package v1alpha1

import (
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	conditionsapi "github.com/kube-bind/kube-bind/pkg/apis/third_party/conditions/apis/conditions/v1alpha1"
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
// +kubebuilder:printcolumn:name="Established",type="string",JSONPath=`.status.conditions[?(@.type=="Established")].status`,priority=5
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=`.metadata.creationTimestamp`,priority=0
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
// +kubebuilder:validation:XValidation:rule=`self.scope == "Namespaced" || self.informerScope == "Cluster"`,message="informerScope is must be Cluster for cluster-scoped resources"
type APIServiceExportSpec struct {
	APIServiceExportCRDSpec `json:",inline"`

	// +optional
	// +listType=map
	// +listMapKey=group
	// +listMapKey=resource
	// +kubebuilder:validation:MaxItems=2
	PermissionClaims []PermissionClaim `json:"permissionClaims,omitempty"`

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
	InformerScope Scope `json:"informerScope"`
}

type APIServiceExportCRDSpec struct {
	// group is the API group of the defined custom resource. Empty string means the
	// core API group. 	The resources are served under `/apis/<group>/...` or `/api` for the core group.
	//
	// +required
	Group string `json:"group"`

	// names specify the resource and kind names for the custom resource.
	//
	// +required
	Names apiextensionsv1.CustomResourceDefinitionNames `json:"names"`

	// scope indicates whether the defined custom resource is cluster- or namespace-scoped.
	// Allowed values are `Cluster` and `Namespaced`.
	//
	// +required
	// +kubebuilder:validation:Enum=Cluster;Namespaced
	Scope apiextensionsv1.ResourceScope `json:"scope"`

	// versions is the API version of the defined custom resource.
	//
	// Note: the OpenAPI v3 schemas must be equal for all versions until CEL
	//       version migration is supported.
	//
	// +required
	// +listType=map
	// +listMapKey=name
	// +kubebuilder:validation:MinItems=1
	Versions []APIServiceExportVersion `json:"versions"`
}

// APIServiceExportVersion describes one API version of a resource.
type APIServiceExportVersion struct {
	// name is the version name, e.g. “v1”, “v2beta1”, etc.
	// The custom resources are served under this version at `/apis/<group>/<version>/...` if `served` is true.
	//
	// +required
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:Pattern=^v[1-9][0-9]*([a-z]+[1-9][0-9]*)?$
	Name string `json:"name"`
	// served is a flag enabling/disabling this version from being served via REST APIs
	//
	// +required
	// +kubebuilder:validation:Required
	// +kubebuilder:default=true
	Served bool `json:"served"`
	// storage indicates this version should be used when persisting custom resources to storage.
	// There must be exactly one version with storage=true.
	//
	// +required
	// +kubebuilder:validation:Required
	Storage bool `json:"storage"`
	// deprecated indicates this version of the custom resource API is deprecated.
	// When set to true, API requests to this version receive a warning header in the server response.
	// Defaults to false.
	//
	// +optional
	Deprecated bool `json:"deprecated,omitempty"`
	// deprecationWarning overrides the default warning returned to API clients.
	// May only be set when `deprecated` is true.
	// The default warning indicates this version is deprecated and recommends use
	// of the newest served version of equal or greater stability, if one exists.
	//
	// +optional
	DeprecationWarning *string `json:"deprecationWarning,omitempty"`
	// schema describes the structural schema used for validation, pruning, and defaulting
	// of this version of the custom resource.
	//
	// +required
	// +kubebuilder:validation:Required
	Schema APIServiceExportSchema `json:"schema"`
	// subresources specify what subresources this version of the defined custom resource have.
	//
	// +optional
	Subresources apiextensionsv1.CustomResourceSubresources `json:"subresources,omitempty"`
	// additionalPrinterColumns specifies additional columns returned in Table output.
	// See https://kubernetes.io/docs/reference/using-api/api-concepts/#receiving-resources-as-tables for details.
	// If no columns are specified, a single column displaying the age of the custom resource is used.
	//
	// +optional
	// +listType=map
	// +listMapKey=name
	AdditionalPrinterColumns []apiextensionsv1.CustomResourceColumnDefinition `json:"additionalPrinterColumns,omitempty"`
}

type APIServiceExportSchema struct {
	// openAPIV3Schema is the OpenAPI v3 schema to use for validation and pruning.
	//
	// +kubebuilder:pruning:PreserveUnknownFields
	// +structType=atomic
	// +required
	// +kubebuilder:validation:Required
	OpenAPIV3Schema runtime.RawExtension `json:"openAPIV3Schema"`
}

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
}

// APIServiceExportList is the objects list that represents the APIServiceExport.
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type APIServiceExportList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []APIServiceExport `json:"items"`
}
