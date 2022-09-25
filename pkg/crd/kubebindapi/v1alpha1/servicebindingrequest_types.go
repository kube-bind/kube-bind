/*
Copyright 2022 The Kubectl Bind API contributors.

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

//+genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:shortName=sbr

// ServiceBindingRequest is the object that represents the ServiceBindingRequest.
type ServiceBindingRequest struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// ServiceBindingRequestSpec represents the service binding requestSpec spec
	ServiceBindingRequestSpec ServiceBindingRequestSpec `json:"serviceBindingRequestSpec"`
}

// ServiceBindingRequestSpec represents the data in the newly created ServiceBindingRequest
type ServiceBindingRequestSpec struct {
	// AuthenticatedClientURL is the service provider url where the service binding request is made, e.g: www.mangodb.com/kubernetes.
	AuthenticatedClientURL string `json:"authenticatedClientURL"`
	// SessionID is a unique value of each session made to the backend server. In general, this value should be the state
	// in the oauth request as well. One example could be the terminal(command line) session opened.
	SessionID string `json:"sessionID"`
	// UserID Probably we don't need however it could be a convenient value for better computations.
	UserID string `json:"userID"`
	// UserEmail is the email address that is used by the user to register at the service provider. This is acquired and
	// saved after the request has been made and probably approved.
	UserEmail string `json:"userEmail"`
	// Approved represents the status of the request, whether it has been approved or not.
	Approved bool `json:"approved"`
	// OIDCRequestSpec contains the request data of the OIDC request needed to establish the connection to the authorization.
	// We might not need this I am just adding it for better clarity of how the api should work.
	// Could br also abstracted into a new type.
	OIDCRequestSpec *OIDCRequestSpec `json:"oidcRequestSpec,omitempty"`
	// OIDCResponseSpec contains the response of the OIDC request which has the all needed data/credentials to access the
	// authorization server to get all the user related data.
	OIDCResponseSpec *OIDCResponseSpec `json:"oidcResponseSpec,omitempty"`
}

// OIDCRequestSpec contains the request data of the OIDC request needed to establish the connection to the authorization.
type OIDCRequestSpec struct {
	IssuerURL   string   `json:"issuerURL"`
	ConnectorID string   `json:"connectorID,omitempty"`
	Scopes      []string `json:"scopes,omitempty"`
}

// OIDCResponseSpec contains the response of the OIDC request which has the all needed data/credentials to access the
// authorization server to get all the user related data.
type OIDCResponseSpec struct {
	TokenID      string `json:"tokenID,omitempty"`
	AccessToken  string `json:"accessToken,omitempty"`
	RefreshToken string `json:"refreshToken,omitempty"`
	Claims       string `json:"claims,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ServiceBindingRequestList is the objects list that represents the ServiceBindingRequest.
type ServiceBindingRequestList struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Items []ServiceBindingRequestSpec `json:"items"`
}
