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
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	ResourceKindResourceQuery = "ResourceQuery"
	ResourceResourceQuery     = "resourcequery"
	ResourceResourceQueries   = "resourcequeries"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=resourcequeries,singular=resourcequery,scope=Cluster
type ResourceQuery struct {
	metav1.TypeMeta `json:",inline"`
	// Request describes the attributes for the graph request.
	// +optional
	Request *ResourceQueryRequest `json:"request,omitempty"`
	// Response describes the attributes for the graph response.
	// +optional
	Response *runtime.RawExtension `json:"response,omitempty"`
}

type ResourceQueryRequest struct {
	Source SourceInfo `json:"source"`
	// +optional
	Target       *shared.ResourceLocator `json:"target,omitempty"`
	OutputFormat OutputFormat            `json:"outputFormat"`
}

type SourceInfo struct {
	Resource  kmapi.ResourceID `json:"resource" protobuf:"bytes,1,opt,name=resource"`
	Namespace string           `json:"namespace"`
	// +optional
	Name     string                `json:"name,omitempty"`
	Selector *metav1.LabelSelector `json:"selector,omitempty"`
}

// +kubebuilder:validation:Enum=Ref;Object;Table
type OutputFormat string

const (
	OutputFormatRef    OutputFormat = "Ref"
	OutputFormatObject OutputFormat = "Object"
	OutputFormatTable  OutputFormat = "Table"
)
