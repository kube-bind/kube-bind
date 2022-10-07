/*
Copyright 2022 The Kubectl Bind contributors.

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

//+genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:shortName=sbr
// +kubebuilder:subresource:status

// ServiceBindingRequest is the object that represents the ServiceBindingRequest.
type ServiceBindingRequest struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec represents the service binding requestSpec spec
	ServiceBindingRequestSpec ServiceBindingRequestSpec `json:"spec"`

	// status contains reconciliation information for the service binding request.
	Status ServiceBindingRequestStatus `json:"status,omitempty"`
}

// ServiceBindingRequestSpec represents the data in the newly created ServiceBindingRequest
type ServiceBindingRequestSpec struct {
	// authenticatedClientURL is the service provider url where the service binding request is made, e.g: www.mangodb.com/kubernetes.
	AuthenticatedClientURL string `json:"authenticatedClientURL"`
	// sessionID is a unique value of each session made to the backend server. In general, this value should be the state
	// in the oauth request as well. One example could be the terminal(command line) session opened.
	SessionID string `json:"sessionID"`
	// userID Probably we don't need however it could be a convenient value for better computations.
	UserID string `json:"userID"`
	// UserEmail is the email address that is used by the user to register at the service provider. This is acquired and
	// saved after the request has been made and probably approved.
	UserEmail string `json:"userEmail"`
	// OIDCRequestSpec contains the request data of the OIDC request needed to establish the connection to the authorization.
	// We might not need this I am just adding it for better clarity of how the api should work.
	// Could br also abstracted into a new type.
	OIDCRequestSpec *OIDCRequestSpec `json:"oidcRequestSpec,omitempty"`
	// OIDCResponseSpec contains the response of the OIDC request which has the all needed data/credentials to access the
	// authorization server to get all the user related data.
	OIDCResponseSpec *OIDCResponseSpec `json:"oidcResponseSpec,omitempty"`
	// serviceProviderSpec contains all the data the service provider needs to conduct the chosen service by the user.
	//An exmaple of those specs could be the resources that the user has chosen to use.
	ServiceProviderSpec runtime.RawExtension `json:"serviceProviderSpecSpec,omitempty"`
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

// ServiceBindingRequestStatus stores status information about a service binding request.
type ServiceBindingRequestStatus struct {
	// +optional
	LastUpdated metav1.Time `json:"lastUpdated,omitempty"`

	// approved represents the status of the request, whether it has been approved or not.
	//  +optional
	Approved bool `json:"approved"`

	// TODO: use conditions instead of Error messages.https://github.com/kcp-dev/kcp/blob/main/pkg/apis/third_party/conditions/apis/conditions/v1alpha1/types.go
	// errorMessage contains a default error message in case the controller encountered an error.
	// Will be reset if the error was resolved.
	// +optional
	ErrorMessage *string `json:"errorMessage,omitempty"`

	// errorReason contains a error reason in case the controller encountered an error. Will be reset if the error was resolved.
	// +optional
	ErrorReason string `json:"errorReason,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ServiceBindingRequestList is the objects list that represents the ServiceBindingRequest.
type ServiceBindingRequestList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []ServiceBindingRequest `json:"items"`
}
