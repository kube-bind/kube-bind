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
)

// BindingProvider is a non-CRUD resource that is returned by the server before
// authentication. It specifies which authentication flows the provider supports.
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type BindingProvider struct {
	metav1.TypeMeta `json:",inline"`

	// providerPrettyName is the name of the provider that is displayed to the user, e.g:
	// MangoDB Inc.
	//
	// +required
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	ProviderPrettyName string `json:"providerPrettyName"`

	// authenticationMethods is a list of authentication methods supported by the
	// service provider.
	//
	// +required
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	AuthenticationMethods []AuthenticationMethod `json:"authenticationMethods,omitempty"`
}

type AuthenticationMethod struct {
	// method is the name of the authentication method. The follow methods are supported:
	//
	// - "OAuth2CodeGrant"
	//
	// The list is ordered by preference by the service provider. The consumer should
	// try to use the first method in the list that matches the capabilities of the
	// client environment (e.g. does the client support a web browser) and the
	// flags provided by the user.
	//
	// +required
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=OAuth2CodeGrant
	Method string `json:"method,omitempty"`

	// OAuth2CodeGrant is the configuration for the OAuth2 code grant flow.
	OAuth2CodeGrant *OAuth2CodeGrant `json:"oauth2CodeGrant,omitempty"`
}

type OAuth2CodeGrant struct {
	// authenticatedURL is the service provider url that the service consumer will use to authenticate against
	// the service provider in case of using OIDC mode made, e.g: www.mangodb.com/kubernetes/authorize.
	//
	// +required
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	AuthenticatedURL string `json:"authenticatedURL"`
}
