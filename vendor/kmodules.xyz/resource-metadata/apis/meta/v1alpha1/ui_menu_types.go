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

	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	ResourceKindMenu = "Menu"
	ResourceMenu     = "menu"
	ResourceMenus    = "menus"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=menus,singular=menu,scope=Cluster
type Menu struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              MenuSpec `json:"spec,omitempty"`
}

type MenuSpec struct {
	Mode MenuMode `json:"mode"`
	// +optional
	Home *MenuSectionInfo `json:"home,omitempty"`
	// +optional
	Sections []*MenuSection `json:"sections,omitempty"`
}

type MenuSection struct {
	MenuSectionInfo `json:",inline,omitempty"`
	Items           []MenuItem `json:"items"`
}

type MenuSectionInfo struct {
	Name string `json:"name,omitempty"`

	// +optional
	Path string `json:"path,omitempty"`
	// +optional
	APIGroup string `json:"apiGroup,omitempty"`

	// +optional
	LayoutName string `json:"layoutName,omitempty"`

	// Icons is an optional list of icons for an application. Icon information includes the source, size,
	// and mime type.
	Icons []shared.ImageSpec `json:"icons,omitempty"`
}

type MenuItem struct {
	Name string `json:"name"`
	// +optional
	Path     string            `json:"path,omitempty"`
	Resource *kmapi.ResourceID `json:"resource,omitempty"`
	Missing  bool              `json:"missing,omitempty"`
	// +optional
	Required bool `json:"required,omitempty"`
	// +optional
	LayoutName string `json:"layoutName,omitempty"`
	// +optional
	Icons     []shared.ImageSpec           `json:"icons,omitempty"`
	Installer *shared.DeploymentParameters `json:"installer,omitempty"`
	// +optional
	Preset *core.TypedLocalObjectReference `json:"preset,omitempty"`
	// +optional
	AvailableVariants int `json:"availableVariants,omitempty"`
	// +optional
	Items []MenuItem `json:"items,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true

type MenuList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Menu `json:"items,omitempty"`
}
