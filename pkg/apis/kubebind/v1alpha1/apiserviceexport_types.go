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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	conditionsapi "github.com/kube-bind/kube-bind/pkg/apis/third_party/conditions/apis/conditions/v1alpha1"
)

const (
	// APIServiceExportConditionConnected means the APIServiceExport has been connected to a APIServiceBinding.
	APIServiceExportConditionConnected conditionsapi.ConditionType = "Connected"

	// APIServiceExportConditionServiceBindingReady is set to true when the APIServiceExport is ready.
	APIServiceExportConditionServiceBindingReady conditionsapi.ConditionType = "ExportReady"

	// APIServiceExportConditionResourcesValid is set to true when the APIServiceExport's
	// resources exist and are valid.
	APIServiceExportConditionResourcesValid conditionsapi.ConditionType = "ResourcesValid"

	// APIServiceExportConditionSchemaInSync is set to true when the APIServiceExport's
	// schema is applied to the consumer cluster.
	APIServiceExportConditionSchemaInSync conditionsapi.ConditionType = "SchemaInSync"

	// APIServiceExportConditionResourcesInSync is set to true when the APIServiceExport's
	// resources are in sync with the CRDs.
	APIServiceExportConditionResourcesInSync conditionsapi.ConditionType = "ResourcesInSync"
)

// APIServiceExport specifies an API service to exported to a consumer cluster. The
// consumer cluster is defined by the ClusterBinding singleton in the same namespace.
//
// +crd
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:scope=Namespaced,categories=kube-bindings
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=`.status.conditions[?(@.type=="Ready")].status`,priority=0
type APIServiceExport struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec represents the data in the newly created service binding export.
	//
	// +required
	// +kubebuilder:validation:Required
	Spec APIServiceExportSpec `json:"spec"`

	// status contains reconciliation information for the service binding export.
	Status APIServiceExportStatus `json:"status,omitempty"`
}

func (in *APIServiceExport) GetConditions() conditionsapi.Conditions {
	return in.Status.Conditions
}

func (in *APIServiceExport) SetConditions(conditions conditionsapi.Conditions) {
	in.Status.Conditions = conditions
}

type APIServiceExportSpec struct {
	// resources are the resources to be bound into the consumer cluster.
	//
	// +listType=map
	// +listMapKey=group
	// +listMapKey=resource
	Resources []APIServiceExportGroupResource `json:"resources,omitempty"`

	// scope is the scope of the APIServiceExport. It can be either Cluster or Namespace.
	//
	// Cluster:    The konnector has permission to watch all namespaces at once and cluster-scoped resources.
	//             This is more efficient than watching each namespace individually.
	// Namespaced: The konnector has permission to watch only single namespaces.
	//             This is more resource intensive. And it means cluster-scoped resources cannot be exported.
	//
	// +required
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:XValidation:rule="self != \"Namespaced\"",message="Namespaced scope not yet supported"
	Scope Scope `json:"scope"`
}

type APIServiceExportStatus struct {
	// conditions is a list of conditions that apply to the APIServiceExport.
	Conditions conditionsapi.Conditions `json:"conditions,omitempty"`
}

type APIServiceExportGroupResource struct {
	GroupResource `json:",inline"`
}

// GroupResource identifies a resource.
type GroupResource struct {
	// group is the name of an API group.
	// For core groups this is the empty string '""'.
	//
	// +kubebuilder:validation:Pattern=`^(|[a-z0-9]([-a-z0-9]*[a-z0-9](\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*)?)$`
	// +kubebuilder:default=""
	Group string `json:"group,omitempty"`

	// resource is the name of the resource.
	// Note: it is worth noting that you can not ask for permissions for resource provided by a CRD
	// not provided by an service binding export.
	//
	// +kubebuilder:validation:Pattern=`^[a-z][-a-z0-9]*[a-z0-9]$`
	// +required
	// +kubebuilder:validation:Required
	Resource string `json:"resource"`
}

// APIServiceExportList is the objects list that represents the APIServiceExport.
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type APIServiceExportList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []APIServiceExport `json:"items"`
}
