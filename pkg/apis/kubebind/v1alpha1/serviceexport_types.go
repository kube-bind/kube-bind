/*
Copyright 2022 The Kubectl Bind contributors.

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
	// ServiceExportConditionConnected means the ServiceExport has been connected to a ServiceBinding.
	ServiceExportConditionConnected conditionsapi.ConditionType = "Connected"

	// ServiceExportConditionExportReady is set to true when the ServiceExport is ready.
	ServiceExportConditionExportReady conditionsapi.ConditionType = "ExportReady"

	// ServiceExportConditionResourcesValid is set to true when the ServiceExport's
	// resources exist and are valid.
	ServiceExportConditionResourcesValid conditionsapi.ConditionType = "ResourcesValid"

	// ServiceExportConditionSchemaInSync is set to true when the ServiceExport's
	// schema is applied to the consumer cluster.
	ServiceExportConditionSchemaInSync conditionsapi.ConditionType = "SchemaInSync"
)

// ServiceExport specifies an API service to exported to a consumer cluster. The
// consumer cluster is defined by the ClusterBinding singleton in the same namespace.
//
// +crd
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:scope=Namespaced,categories=kube-bindings
// +kubebuilder:subresource:status
type ServiceExport struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec represents the data in the newly created service binding export.
	//
	// +required
	// +kubebuilder:validation:Required
	Spec ServiceExportSpec `json:"spec"`

	// status contains reconciliation information for the service binding export.
	Status ServiceExportStatus `json:"status,omitempty"`
}

func (in *ServiceExport) GetConditions() conditionsapi.Conditions {
	return in.Status.Conditions
}

func (in *ServiceExport) SetConditions(conditions conditionsapi.Conditions) {
	in.Status.Conditions = conditions
}

type ServiceExportSpec struct {
	// resources are the resources to be bound into the consumer cluster.
	//
	// +listType=map
	// +listMapKey=group
	// +listMapKey=resource
	Resources []ServiceExportGroupResource `json:"resources,omitempty"`
}

type ServiceExportStatus struct {
	// conditions is a list of conditions that apply to the ServiceExport.
	Conditions conditionsapi.Conditions `json:"conditions,omitempty"`
}

type ServiceExportGroupResource struct {
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

// ServiceExportList is the objects list that represents the ServiceExport.
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ServiceExportList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []ServiceExport `json:"items"`
}
