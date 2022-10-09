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

	conditionsapi "github.com/kube-bind/kube-bind/pkg/apis/third_party/conditions/apis/conditions/v1alpha1"
)

// ClusterBinding represents a bound consumer class. It lives in a service provider cluster
// and is a singleton named "cluster".
//
// +crd
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:scope=Namespaced
// +kubebuilder:subresource:status
type ClusterBinding struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec represents the data in the newly created ClusterBinding.
	// +required
	Spec ClusterBindingSpec `json:"spec"`

	// status contains reconciliation information for the service binding.
	// +optional
	Status ClusterBindingStatus `json:"status,omitempty"`
}

// ClusterBindingSpec represents the data in the newly created ClusterBinding.
type ClusterBindingSpec struct {
	// kubeconfigSecretName is the secret ref that contains the kubeconfig of the service cluster.
	// +required
	// +kubebuilder:validation:XValidation:rule=="self == oldSelf",message="kubeconfigSecretRef is immutable"
	KubeconfigSecretRef LocalSecretKeyRef `json:"kubeconfigSecretRef"`

	// providerPrettyName is the pretty name of the service provider cluster. This
	// can be shared among different ServiceBindings.
	// +optional
	// +kubebuilder:default="unknown"
	ProviderPrettyName string `json:"providerPrettyName,omitempty"`

	// serviceProviderSpec contains all the data and information about the service which has been bound to the service
	// binding request. The service providers decide what they need and what to configure based on what then include in
	// this field, such as service region, type, tiers, etc...
	// +optional
	ServiceProviderSpec runtime.RawExtension `json:"serviceProviderSpec,omitempty"`
}

// ClusterBindingPhase stores the phase of a cluster binding.
//
// +kubebuilder:validation:Enum=Connected;Pending;Timeout
type ClusterBindingPhase string

const (
	// ClusterConnected means the service is connected and has sent a heartbeat recently.
	ClusterConnected ClusterBindingPhase = "Connected"
	// ClusterPending is the phase before the konnector has sent a heartbeat the first time.
	ClusterPending ClusterBindingPhase = "Pending"
	// ClusterTimeout is the phase when the konnector has not sent a heartbeat for a long time
	// and the service considers this cluster as unhealthy.
	ClusterTimeout ClusterBindingPhase = "Timeout"
)

// ClusterBindingStatus stores status information about a service binding. It is
// updated by both the konnector and the service provider.
type ClusterBindingStatus struct {
	// lastHeartbeatTime is the last time the konnector updated the status.
	// +optional
	LastHeartbeatTime metav1.Time `json:"lastHeartbeatTime,omitempty"`

	// heartbeatInterval is the maximal interval between heartbeats that the
	// konnector promises to send. The service provider can assume that the
	// konnector is not unhealthy if it does not receive a heartbeat within
	// this time.
	//
	// +optional
	HeartbeatInterval metav1.Duration `json:"heartbeatInterval,omitempty"`

	// phase represents the phase of the service binding. It is set by the
	// service provider.
	//
	// +optional
	// +kubebuilder:default=Pending
	Phase ClusterBindingPhase `json:"phase"`

	// conditions is a list of conditions that apply to the ClusterBinding. It is
	// updated by the konnector and the service provider.
	//
	// +optional
	Conditions conditionsapi.Conditions `json:"conditions,omitempty"`
}

// ClusterBindingList is the objects list that represents the ClusterBinding.
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ClusterBindingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []ClusterBinding `json:"items"`
}
