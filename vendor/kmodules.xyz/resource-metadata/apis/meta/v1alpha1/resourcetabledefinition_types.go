/*
Copyright AppsCode Inc. and Contributors

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
	kmapi "kmodules.xyz/client-go/api/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	ResourceKindResourceTableDefinition = "ResourceTableDefinition"
	ResourceResourceTableDefinition     = "resourcetabledefinition"
	ResourceResourceTableDefinitions    = "resourcetabledefinitions"
)

// +genclient
// +genclient:nonNamespaced
// +genclient:skipVerbs=updateStatus
// +k8s:openapi-gen=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=resourcetabledefinitions,singular=resourcetabledefinition,scope=Cluster
type ResourceTableDefinition struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              ResourceTableDefinitionSpec `json:"spec,omitempty"`
}

type ResourceTableDefinitionSpec struct {
	Resource    *kmapi.ResourceID          `json:"resource,omitempty"`
	DefaultView bool                       `json:"defaultView"`
	Columns     []ResourceColumnDefinition `json:"columns,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true

type ResourceTableDefinitionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ResourceTableDefinition `json:"items,omitempty"`
}
