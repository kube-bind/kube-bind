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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kubebindv1alpha2 "github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2"
	conditionsapi "github.com/kube-bind/kube-bind/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
)

// Module groups multiple CRDs with related resources (permissionClaims) as a Service definition.
//
// +crd
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:scope=Cluster,categories=kube-bind
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Resources",type="string",JSONPath=`.spec.resources[*].group`
// +kubebuilder:printcolumn:name="PermissionClaims",type="string",JSONPath=`.spec.permissionClaims[*].resource`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=`.metadata.creationTimestamp`,priority=0
type Module struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec specifies the module.
	// +required
	// +kubebuilder:validation:Required
	Spec ModuleSpec `json:"spec"`

	// status contains reconciliation information for the module.
	Status ModuleStatus `json:"status,omitempty"`
}

func (in *Module) GetConditions() conditionsapi.Conditions {
	return in.Status.Conditions
}

func (in *Module) SetConditions(conditions conditionsapi.Conditions) {
	in.Status.Conditions = conditions
}

// ModuleSpec defines the desired state of Module.
type ModuleSpec struct {
	// scope defines the scope of the resources in this module.
	// +required
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=Namespaced;Cluster
	Scope kubebindv1alpha2.InformerScope `json:"scope,omitempty"`
	// resources defines the CRDs that are part of this module.
	//
	// +required
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	Resources []kubebindv1alpha2.APIServiceExportResource `json:"resources"`

	// permissionClaims defines the permission claims required by this module.
	//
	// +optional
	PermissionClaims []kubebindv1alpha2.PermissionClaim `json:"permissionClaims,omitempty"`

	// namespaces specifies the namespaces that should be bootstrapped as part of this module.
	// When objects originate from provider side, consumer does not always know the necessary details.
	// This field allows provider to pre-heat the necessary namespaces on provider side by creating
	// APIServiceNamespace objects attached to the APIServiceExport. More namespaces can be created later by the consumer.
	//
	// +optional
	Namespaces []kubebindv1alpha2.Namespaces `json:"namespaces,omitempty"`
}

// ModuleStatus stores status information about a Module.
type ModuleStatus struct {
	// conditions is a list of conditions that apply to the Module.
	Conditions conditionsapi.Conditions `json:"conditions,omitempty"`
}

// ModuleList is the list of Module.
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ModuleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Module `json:"items"`
}
