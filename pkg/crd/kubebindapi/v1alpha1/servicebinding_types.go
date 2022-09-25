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
	"k8s.io/apimachinery/pkg/runtime"
)

//+genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:shortName=sb
// +kubebuilder:subresource:status

// ServiceBinding is the object that represents the ServiceBinding.
type ServiceBinding struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// ServiceBindingSpec represents the data in the newly created ServiceBinding.
	ServiceBindingSpec ServiceBindingSpec `json:"serviceBindingSpec"`

	// Status contains reconciliation information for the service binding.
	Status ServiceBindingStatus `json:"status,omitempty"`
}

// ServiceBindingSpec represents the data in the newly created ServiceBinding.
type ServiceBindingSpec struct {
	// ServiceBindingRequestName represents the name of the service binding request in order to access the any needed
	// field or property for service binding.
	ServiceBindingRequestName string `json:"serviceBindingRequestName"`
	// KubeconfigSecretName is the name of the secret that contains the kubeconfig of the service cluster.
	KubeconfigSecretName string `json:"kubeconfigSecretName"`
	// ServiceProviderSpec contains all the data and information about the service which has been bound to the service
	// binding request. The service providers decide what they need and what to configure based on what then include in
	// this field, such as service region, type, tiers, etc...
	ServiceProviderSpec runtime.RawExtension `json:"serviceProviderSpec"`
}

// +kubebuilder:validation:Enum=Connected;Pending;Expired

type ServiceBindingPhase string

const (
	ServiceConnected ServiceBindingPhase = "Connected"
	ServicePending                       = "Pending"
	ServiceExpired                       = "Expired"
)

// ServiceBindingStatus stores status information about a service binding.
type ServiceBindingStatus struct {
	// +optional
	LastUpdated metav1.Time `json:"lastUpdated,omitempty"`

	// Phase represents the phase of the service binding.
	//  +optional
	Phase ServiceBindingPhase `json:"phase"`

	// ErrorMessage contains a default error message in case the controller encountered an error.
	// Will be reset if the error was resolved.
	// +optional
	ErrorMessage *string `json:"errorMessage,omitempty"`

	// ErrorReason contains a error reason in case the controller encountered an error. Will be reset if the error was resolved.
	// +optional
	ErrorReason string `json:"errorReason,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ServiceBindingList is the objects list that represents the ServiceBinding.
type ServiceBindingList struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Items []ServiceBinding `json:"items"`
}
