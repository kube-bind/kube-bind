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

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// ServiceBindingSession is the object that represents the a connection session between the client and the service provider.
//
// +crd
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:shortName=sbs
// +kubebuilder:subresource:status
type ServiceBindingSession struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec represents the service binding session requestSpec spec
	Spec ServiceBindingSessionSpec `json:"spec"`

	// status contains reconciliation information for the service binding request.
	Status ServiceProviderStatus `json:"status,omitempty"`
}

// ServiceBindingSessionSpec represents the data in the newly created ServiceBindingSession
type ServiceBindingSessionSpec struct {
	// kubeconfigSecretName is the secret ref that contains the kubeconfig of the service cluster.
	// +required
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="kubeconfigSecretRef is immutable"
	KubeconfigSecretRef LocalSecretKeyRef `json:"kubeconfigSecretRef"`

	// SessionID is the session id that was used by the kubectl bind command that distinguishes the current connection.
	SessionID string `json:"sessionIDz"`

	// AccessToken is the token that kubectl bind might use in the future when binding to another service from the same
	// service provider
	AccessTokenRef LocalSecretKeyRef `json:"accessToken"`
}

// ServiceBindingSessionStatus stores status information about a service binding session request.
type ServiceBindingSessionStatus struct {
	LastUpdated metav1.Time `json:"lastUpdated,omitempty"`

	// errorMessage contains a default error message in case the controller encountered an error.
	// Will be reset if the error was resolved.
	ErrorMessage *string `json:"errorMessage,omitempty"`

	// errorReason contains a error reason in case the controller encountered an error. Will be reset if the error was resolved.
	ErrorReason string `json:"errorReason,omitempty"`
}
