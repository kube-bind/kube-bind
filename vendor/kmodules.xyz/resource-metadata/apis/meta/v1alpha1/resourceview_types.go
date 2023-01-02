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

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type ResourceView struct {
	Resource   kmapi.ResourceID     `json:"resource"`
	LayoutName string               `json:"layoutName"`
	Header     *PageBlockView       `json:"header,omitempty"`
	TabBar     *PageBlockView       `json:"tabBar,omitempty"`
	Pages      []ResourcePageView   `json:"pages,omitempty"`
	UI         *shared.UIParameters `json:"ui,omitempty"`
}

type ResourcePageView struct {
	Name    string          `json:"name"`
	Info    *PageBlockView  `json:"info,omitempty"`
	Insight *PageBlockView  `json:"insight,omitempty"`
	Blocks  []PageBlockView `json:"blocks,omitempty"`
}

type PageBlockView struct {
	Kind    TableKind        `json:"kind"` // Connection | Subtable(Field)
	Name    string           `json:"name,omitempty"`
	Actions *ResourceActions `json:"actions,omitempty"`

	Resource *kmapi.ResourceID `json:"resource,omitempty"`
	// +optional
	Items []unstructured.Unstructured `json:"items,omitempty"`
	// +optional
	Table  *Table       `json:"table,omitempty"`
	Result RenderResult `json:"result"`
}

// +kubebuilder:validation:Enum=Success;Missing;Error
type RenderStatus string

const (
	RenderSuccess RenderStatus = "Success"
	RenderMissing RenderStatus = "Missing"
	RenderError   RenderStatus = "Error"
)

// Result contains the results of a call to compute the status of
// a resource.
type RenderResult struct {
	// Status
	Status RenderStatus `json:"status"`
	// Message
	Message string `json:"message,omitempty"`
}
