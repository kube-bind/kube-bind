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

// APIServiceExportTemplate groups multiple CRDs with related resources (permissionClaims) as a Service definition.
// It is used by a web UI or CLI to allow users to select a set of resources to export from provider cluster to consumer cluster.
// This object is considered a static asset on the provider side and is not expected to change frequently.
//
// +crd
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:scope=Cluster,categories=kube-bind
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Resources",type="string",JSONPath=`.spec.resources[*].group`
// +kubebuilder:printcolumn:name="PermissionClaims",type="string",JSONPath=`.spec.permissionClaims[*].resource`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=`.metadata.creationTimestamp`,priority=0
type APIServiceExportTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec specifies the template.
	// +required
	// +kubebuilder:validation:Required
	Spec APIServiceExportTemplateSpec `json:"spec"`

	// status contains reconciliation information for the template.
	Status APIServiceExportTemplateStatus `json:"status,omitempty"`
}

func (in *APIServiceExportTemplate) GetConditions() conditionsapi.Conditions {
	return in.Status.Conditions
}

func (in *APIServiceExportTemplate) SetConditions(conditions conditionsapi.Conditions) {
	in.Status.Conditions = conditions
}

// APIServiceExportTemplateSpec defines the desired state of APIServiceExportTemplate.
type APIServiceExportTemplateSpec struct {
	// description is an optional description of the template.
	//
	// +optional
	Description string `json:"description,omitempty"`
	// scope defines the scope of the resources in this template.
	// +required
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=Namespaced;Cluster
	Scope InformerScope `json:"scope,omitempty"`
	// resources defines the CRDs that are part of this template.
	//
	// +required
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	Resources []APIServiceExportResource `json:"resources"`

	// permissionClaims defines the permission claims required by this template.
	//
	// +optional
	PermissionClaims []PermissionClaim `json:"permissionClaims,omitempty"`

	// Namespaces specifies the namespaces that should be bootstrapped as part of this template.
	// When objects originate from provider side, consumer does not always know the necessary details
	// This field allows provider to pre-heat the necessary namespaces on provider side by creating
	// APIServiceNamespace objects attached to the APIServiceExport. More namespaces can be created later by the consumer.
	//
	// +optional
	// +kubebuilder:validation:Optional
	Namespaces []Namespaces `json:"namespaces,omitempty"`
}

// APIServiceExportTemplateStatus stores status information about an APIServiceExportTemplate.
type APIServiceExportTemplateStatus struct {
	// conditions is a list of conditions that apply to the APIServiceExportTemplate.
	Conditions conditionsapi.Conditions `json:"conditions,omitempty"`
}

// APIServiceExportTemplateList is the list of APIServiceExportTemplate.
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type APIServiceExportTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []APIServiceExportTemplate `json:"items"`
}
