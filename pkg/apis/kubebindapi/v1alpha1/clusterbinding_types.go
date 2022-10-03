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
	conditionsv1alpha1 "github.com/kube-bind/kube-bind-api/pkg/apis/third_party/conditions/apis/conditions/v1alpha1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

//+genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:shortName=sb
// +kubebuilder:subresource:status

// ClusterBinding is the object that represents the ClusterBinding.
type ClusterBinding struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec represents the data in the newly created ClusterBinding.
	Spec ClusterBindingSpec `json:"spec"`

	// Status contains reconciliation information for the service binding.
	Status ClusterBindingStatus `json:"status,omitempty"`
}

func (in *ClusterBinding) GetConditions() conditionsv1alpha1.Conditions {
	return in.Status.Conditions
}

func (in *ClusterBinding) SetConditions(conditions conditionsv1alpha1.Conditions) {
	in.Status.Conditions = conditions
}

// ClusterBindingSpec represents the data in the newly created ClusterBinding.
type ClusterBindingSpec struct {
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

// ClusterBindingStatus stores status information about a service binding.
type ClusterBindingStatus struct {
	// +optional
	LastUpdated metav1.Time `json:"lastUpdated,omitempty"`

	// Phase represents the phase of the service binding.
	//  +optional
	Phase ServiceBindingPhase `json:"phase"`

	// conditions is a list of conditions that apply to the APIBinding.
	//
	// +optional
	Conditions conditionsv1alpha1.Conditions `json:"conditions,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterBindingList is the objects list that represents the ClusterBinding.
type ClusterBindingList struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Items []ClusterBinding `json:"items"`
}
