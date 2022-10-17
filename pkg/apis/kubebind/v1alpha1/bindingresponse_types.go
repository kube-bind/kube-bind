/*
Copyright 2022 The Kube Bind Authors.

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
	"k8s.io/apimachinery/pkg/runtime"
)

// BindingResponse is a non-CRUD resource that is returned by the server after
// authentication and resource selection on the service prpvider website. It returns
// a list of requests of possibly different types that kubectl bind has to
// pass to the sub-command kubect-bind-<type>, e.g. kubectl-bind-apiservice for
// APIServiceBindingRequest.
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type BindingResponse struct {
	metav1.TypeMeta `json:",inline"`

	// authentication is data specific to the  authentication method that was used.
	//
	// +optional
	// +kubebuilder:validation:Optional
	Authentication *runtime.RawExtension `json:"authentication,omitempty"`

	// kubeconfig is a kubeconfig file that can be used to access the service provider cluster.
	//
	// +required
	// +kubebuilder:validation:Required
	Kubeconfig []byte `json:"kubeconfig"`

	// requests is a list of binding requests of different types, e.g. APIServiceBindingRequest.
	//
	// +required
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	Requests []runtime.RawExtension `json:"requests"`
}
