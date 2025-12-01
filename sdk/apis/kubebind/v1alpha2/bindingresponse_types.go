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
	"errors"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation"
)

// BindingResourceResponse is a non-CRUD resource that is returned by the server after
// authentication and resource selection on the service prpvider website. It returns
// a list of requests of possibly different types that kubectl bind has to
// pass to the sub-command kubect-bind-<type>, e.g. kubectl-bind-apiservice for
// APIServiceExportRequest.
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:storageversion
type BindingResourceResponse struct {
	metav1.TypeMeta `json:",inline"`

	// authentication is data specific to the authentication method that was used.
	//
	// +optional
	// +kubebuilder:validation:Optional
	Authentication BindingResponseAuthentication `json:"authentication,omitempty"`

	// kubeconfig is a kubeconfig file that can be used to access the service provider cluster.
	//
	// +required
	// +kubebuilder:validation:Required
	Kubeconfig []byte `json:"kubeconfig"`

	// requests is a list of binding requests of different types, e.g. APIServiceExportRequest.
	//
	// +required
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	Requests []runtime.RawExtension `json:"requests"`
}

// BindingResponseAuthentication is the authentication data specific to the
// authentication method that was used. Exactly one field must be set.
type BindingResponseAuthentication struct {
	// oauth2CodeGrant is the data returned by the OAuth2 code grant flow.
	//
	// +optional
	// +kubebuilder:validation:Optional
	OAuth2CodeGrant *BindingResponseAuthenticationOAuth2CodeGrant `json:"oauth2CodeGrant,omitempty"`
}

// BindingResponseAuthenticationOAuth2CodeGrant contains the authentication data which is passed back to
// the consumer as BindingResponse.Authentication. It is authentication method
// specific.
type BindingResponseAuthenticationOAuth2CodeGrant struct {
	// sessionID is the session ID that was originally passed from the consumer to
	// the service provider. It must be checked to equal the original value.
	SessionID string `json:"sid"`

	// id is the ID of the authenticated user. It is for informational purposes only.
	ID string `json:"id"`
}

// BindableResourcesRequest is sent by the consumer to the service provider
// to indicate which resources the user wants to bind to. It is sent after
// authentication and resource selection on the service provider website.
type BindableResourcesRequest struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	TemplateRef APIServiceExportTemplateRef `json:"templateRef"`
}

type APIServiceExportTemplateRef struct {
	// name is the name of the APIServiceExportTemplate to bind to.
	//
	// +required
	// +kubebuilder:validation:Required
	Name string `json:"name"`
}

func (r *BindableResourcesRequest) Validate() error {
	if r.TemplateRef.Name == "" {
		return errors.New("templateRef.name is required")
	}

	if r.Name == "" {
		return errors.New("name is required")
	}

	// Validate DNS name format for the request name
	if errs := validation.IsDNS1123Label(r.Name); len(errs) > 0 {
		return fmt.Errorf("name %q is not a valid DNS label: %s", r.Name, strings.Join(errs, ", "))
	}

	return nil
}
