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
	"kmodules.xyz/resource-metadata/apis/shared"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	ResourceKindResourceOutline = "ResourceOutline"
	ResourceResourceOutline     = "resourceoutline"
	ResourceResourceOutlines    = "resourceoutlines"
)

// +genclient
// +genclient:nonNamespaced
// +genclient:skipVerbs=updateStatus
// +k8s:openapi-gen=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=resourceoutlines,singular=resourceoutline,scope=Cluster
type ResourceOutline struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              ResourceOutlineSpec `json:"spec,omitempty"`
}

type ResourceOutlineSpec struct {
	Resource      kmapi.ResourceID      `json:"resource"`
	DefaultLayout bool                  `json:"defaultLayout"`
	Header        *PageBlockOutline     `json:"header,omitempty"`
	TabBar        *PageBlockOutline     `json:"tabBar,omitempty"`
	Pages         []ResourcePageOutline `json:"pages,omitempty"`
	// +optional
	Actions []*shared.ActionTemplateGroup `json:"actions,omitempty"`
}

type ResourcePageOutline struct {
	Name    string             `json:"name"`
	Info    *PageBlockOutline  `json:"info,omitempty"`
	Insight *PageBlockOutline  `json:"insight,omitempty"`
	Blocks  []PageBlockOutline `json:"blocks,omitempty"`
}

// +kubebuilder:validation:Enum=Block;Self;SubTable;Connection
type TableKind string

const (
	TableKindBlock      TableKind = "Block"
	TableKindConnection TableKind = "Connection"
	TableKindSubTable   TableKind = "SubTable"
	TableKindSelf       TableKind = "Self"
)

type PageBlockOutline struct {
	Kind                    TableKind `json:"kind"` // ResourceBlockDefinition | Connection | Subtable(Field)
	Name                    string    `json:"name,omitempty"`
	FieldPath               string    `json:"fieldPath,omitempty"`
	*shared.ResourceLocator `json:",inline,omitempty"`
	DisplayMode             ResourceDisplayMode         `json:"displayMode,omitempty"`
	Actions                 *ResourceActions            `json:"actions,omitempty"`
	View                    *ResourceTableDefinitionRef `json:"view,omitempty"`
}

type ResourceTableDefinitionRef struct {
	Name    string                     `json:"name,omitempty"`
	Columns []ResourceColumnDefinition `json:"columns,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true

type ResourceOutlineList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ResourceOutline `json:"items,omitempty"`
}
