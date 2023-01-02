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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	ResourceKindRenderMenu = "RenderMenu"
	ResourceRenderMenu     = "rendermenu"
	ResourceRenderMenus    = "rendermenus"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=rendermenus,singular=rendermenu,scope=Cluster
type RenderMenu struct {
	metav1.TypeMeta `json:",inline"`
	// Request describes the attributes for the render menu request.
	// +optional
	Request *RenderMenuRequest `json:"request,omitempty"`
	// Response describes the attributes for the render menu response.
	// +optional
	Response *Menu `json:"response,omitempty"`
}

type RenderMenuRequest struct {
	Menu string   `json:"menu"`
	Mode MenuMode `json:"mode"`
	// +optional
	Section *string `json:"section,omitempty"`
	// +optional
	Type *metav1.GroupKind `json:"type,omitempty"`
}

// +kubebuilder:validation:Enum=Accordion;Gallery
type MenuMode string

const (
	MenuAccordion MenuMode = "Accordion"
	MenuGallery   MenuMode = "Gallery"
)
