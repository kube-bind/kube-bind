/*
Copyright 2023 The Kube Bind Authors.

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

// APIServiceExportTemplate specifies the resource to be exported.
// It references the CRD to be exported along with additional resources that
// are synchronized from and to the consumer cluster.
//
// +crd
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:scope=Namespaced,categories=kube-bindings
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Established",type="string",JSONPath=`.status.conditions[?(@.type=="Established")].status`,priority=5
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=`.metadata.creationTimestamp`,priority=0
type APIServiceExportTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec specifies the resource.
	// +required
	// +kubebuilder:validation:Required
	Spec APIServiceExportTemplateSpec `json:"spec"`

	// status contains reconciliation information for the resource.
	Status APIServiceExportTemplateStatus `json:"status,omitempty"`
}

type APIServiceExportTemplateSpec struct {
	APIServiceSelector APIServiceSelector `json:"APIServiceSelector"`
	// +optional
	PermissionClaims []PermissionClaim `json:"permissionClaims,omitempty"`
}

type APIServiceExportTemplateStatus struct{}

type APIServiceSelector struct {
	Group    string   `json:"group"`
	Resource string   `json:"resource"`
	Versions []string `json:"versions"`
}

// APIServiceExportRequestList is the list of APIServiceExportRequest.
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type APIServiceExportTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []APIServiceExportTemplate `json:"items"`
}
