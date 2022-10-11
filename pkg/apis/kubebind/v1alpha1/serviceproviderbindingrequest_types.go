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

// ServiceProviderBindingRequest is the object that represents the ServiceProviderBindingRequest.
//
// +crd
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:shortName=sbr
// +kubebuilder:subresource:status
type ServiceProviderBindingRequest struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec represents the service binding requestSpec spec
	Spec ServiceProviderBindingRequestSpec `json:"spec"`

	// status contains reconciliation information for the service binding request.
	Status ServiceProviderBindingRequestStatus `json:"status,omitempty"`
}

// ServiceProviderBindingRequestSpec represents the data in the newly created ServiceProviderBindingRequest
type ServiceProviderBindingRequestSpec struct {
	// AuthenticatedClientURL is the service provider url where the service consumer will use to authenticate against
	// the service provider in case of using OIDC mode made, e.g: www.mangodb.com/kubernetes/authorize.
	//
	// +required
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	AuthenticatedClientURL string `json:"authenticatedClientURL"`

	// providerPrettyName is the pretty name of the service provider where the ServiceBinding is eventually bound. e.g:
	// MongoDB.Inc
	//
	// +required
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	ProviderPrettyName string `json:"providerPrettyName"`

	// serviceProviderSpec contains all the data the service provider needs to conduct the chosen service by the user.
	// An example of those specs could be the resources that the user has chosen to use.
	ServiceProviderSpec runtime.RawExtension `json:"serviceProviderSpecSpec,omitempty"`
}

// ServiceProviderBindingRequestStatus stores status information about a service binding request.
type ServiceProviderBindingRequestStatus struct {
	LastUpdated metav1.Time `json:"lastUpdated,omitempty"`

	// errorMessage contains a default error message in case the controller encountered an error.
	// Will be reset if the error was resolved.
	ErrorMessage *string `json:"errorMessage,omitempty"`

	// errorReason contains a error reason in case the controller encountered an error. Will be reset if the error was resolved.
	ErrorReason string `json:"errorReason,omitempty"`
}

// ServiceProviderBindingRequestList is the objects list that represents the ServiceProviderBindingRequest.
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ServiceProviderBindingRequestList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []ServiceProviderBindingRequest `json:"items"`
}
