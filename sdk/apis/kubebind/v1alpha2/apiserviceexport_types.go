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

	conditionsapi "github.com/kube-bind/kube-bind/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
)

const (
	SourceSpecHashAnnotationKey = "kube-bind.io/source-spec-hash"

	// Label is used to indicate which side owns the resource. By changing the label value,
	// ownership can be transferred.
	ObjectOwnerLabel = "kube-bind.io/owner"
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

	// APIServiceExportConditionPermissionClaim describes status of the permission claim, requested in the APIServiceExport and APIServiceExportRequest.
	APIServiceExportConditionPermissionClaim conditionsapi.ConditionType = "PermissionClaim"
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

// APIServiceExportResource is a type alias for APIServiceExportRequestResource.
type APIServiceExportResource = APIServiceExportRequestResource

// APIServiceExportSpec defines the desired state of APIServiceExport.
type APIServiceExportSpec struct {
	// resources is a list of resources that should be exported.
	//
	// +required
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="resources are immutable"
	Resources []APIServiceExportResource `json:"resources"`

	// PermissionClaims records decisions about permission claims requested by the service provider.
	// Access is granted per GroupResource.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="permissionClaims are immutable"
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
	InformerScope InformerScope `json:"informerScope"`

	// ClusterScopedIsolation specifies how cluster scoped service objects are isolated between multiple consumers on the provider side.
	// It can be "Prefixed", "Namespaced", or "None".
	ClusterScopedIsolation Isolation `json:"clusterScopedIsolation,omitempty"`
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
