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
	ResourceKindRender = "Render"
	ResourceRender     = "render"
	ResourceRenders    = "renders"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=renders,singular=render,scope=Cluster
type Render struct {
	metav1.TypeMeta `json:",inline"`
	// Request describes the attributes for the graph request.
	// +optional
	Request *RenderRequest `json:"request,omitempty"`
	// Response describes the attributes for the graph response.
	// +optional
	Response *RenderResponse `json:"response,omitempty"`
}

type RenderRequest struct {
	Source kmapi.ObjectInfo `json:"source"`

	// If not set, uses DefaultLayout
	LayoutName string           `json:"layoutName,omitempty"`
	PageName   string           `json:"pageName,omitempty"`
	Block      *PageBlockLayout `json:"block,omitempty"`

	ConvertToTable bool        `json:"convertToTable,omitempty"`
	RenderBlocks   []TableKind `json:"renderBlocks,omitempty"`
}

type RenderResponse struct {
	View  *ResourceView  `json:"view,omitempty"`
	Block *PageBlockView `json:"block,omitempty"`
}
