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

// Collection groups multiple APIServiceExportTemplates into a logical group. This functions as a grouping mechanism
// in UIs or CLIs to allow users to select a set of resources to export from provider cluster to consumer cluster.
//
// +crd
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:scope=Cluster,categories=kube-bind
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Description",type="string",JSONPath=`.spec.description`
// +kubebuilder:printcolumn:name="Templates",type="string",JSONPath=`.spec.templates[*].name`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=`.metadata.creationTimestamp`,priority=0
type Collection struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec specifies the collection.
	// +required
	// +kubebuilder:validation:Required
	Spec CollectionSpec `json:"spec"`

	// status contains reconciliation information for the collection.
	Status CollectionStatus `json:"status,omitempty"`
}

func (in *Collection) GetConditions() conditionsapi.Conditions {
	return in.Status.Conditions
}

func (in *Collection) SetConditions(conditions conditionsapi.Conditions) {
	in.Status.Conditions = conditions
}

// CollectionSpec defines the desired state of Collection.
type CollectionSpec struct {
	// description is a human readable description of this collection.
	//
	// +optional
	Description string `json:"description,omitempty"`

	// templates is a list of template references that are part of this collection.
	//
	// +required
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	Templates []APIServiceExportTemplateReference `json:"templates"`
}

// APIServiceExportTemplateReference references an APIServiceExportTemplate by name.
type APIServiceExportTemplateReference struct {
	// name is the name of the template.
	//
	// +required
	// +kubebuilder:validation:Required
	Name string `json:"name"`
}

// CollectionStatus stores status information about a Collection.
type CollectionStatus struct {
	// conditions is a list of conditions that apply to the Collection.
	Conditions conditionsapi.Conditions `json:"conditions,omitempty"`
}

// CollectionList is the list of Collection.
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type CollectionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Collection `json:"items"`
}
