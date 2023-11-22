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

	conditionsapi "github.com/kube-bind/kube-bind/pkg/apis/third_party/conditions/apis/conditions/v1alpha1"
)

const (
	// ClusterBindingConditionSecretValid is set when the secret is valid.
	ClusterBindingConditionSecretValid = "SecretValid"

	// ClusterBindingConditionValidVersion is set when the binary version is valid.
	ClusterBindingConditionValidVersion = "ValidVersion"

	// ClusterBindingConditionHealthy is set when the cluster binding is healthy.
	ClusterBindingConditionHealthy = "Healthy"
)

// ClusterBinding represents a bound consumer class. It lives in a service provider cluster
// and is a singleton named "cluster".
//
// +crd
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:scope=Namespaced,categories=kube-bindings
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Konnector Version",type="string",JSONPath=`.status.konnectorVersion`,priority=0
// +kubebuilder:printcolumn:name="Last Heartbeat",type="date",JSONPath=`.status.lastHeartbeatTime`,priority=0
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=`.status.conditions[?(@.type=="Ready")].status`,priority=0
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=`.metadata.creationTimestamp`,priority=0
// +kubebuilder:validation:XValidation:rule="self.metadata.name == \"cluster\"",message="cluster binding name should be cluster"
type ClusterBinding struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec represents the data in the newly created ClusterBinding.
	// +required
	// +kubebuilder:validation:Required
	Spec ClusterBindingSpec `json:"spec"`

	// status contains reconciliation information for the service binding.
	Status ClusterBindingStatus `json:"status,omitempty"`
}

func (in *ClusterBinding) GetConditions() conditionsapi.Conditions {
	return in.Status.Conditions
}

func (in *ClusterBinding) SetConditions(conditions conditionsapi.Conditions) {
	in.Status.Conditions = conditions
}

// Scope is the scope of the ClusterBinding.
//
// +kubebuilder:validation:Enum=Cluster;Namespaced
type Scope string

const (
	// ClusterScope means that the konnector has permission to watch all namespaces at once.
	// This is more efficient than watching each namespace individually.
	//
	// Only Cluster scoped ClusterBindings can export cluster-scoped resources. But Cluster
	// scoped ClusterBindings can export namespaced resources as well.
	//
	// In a Kubernetes cluster, Cluster scoped ClusterBindings don't allow isolation between tenants.
	// In a kcp cluster, the platform provides the isolation between workspaces and hence tenants
	// are isolated if with full cluster-wide access of a konnector.
	ClusterScope Scope = "Cluster"
	// NamespacedScope means that the konnector has permission to watch only single namespaces.
	// This is more resource intensive than Cluster scope.
	//
	// Only namespace-scoped resources can be exported in Namespace scope.
	NamespacedScope Scope = "Namespaced"
)

// ClusterBindingSpec represents the data in the newly created ClusterBinding.
type ClusterBindingSpec struct {
	// kubeconfigSecretName is the secret ref that contains the kubeconfig of the service cluster.
	//
	// +required
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="kubeconfigSecretRef is immutable"
	KubeconfigSecretRef LocalSecretKeyRef `json:"kubeconfigSecretRef"`

	// providerPrettyName is the pretty name of the service provider cluster. This
	// can be shared among different ServiceBindings.
	//
	// +required
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	ProviderPrettyName string `json:"providerPrettyName"`

	// serviceProviderSpec contains all the data and information about the service which has been bound to the service
	// binding request. The service providers decide what they need and what to configure based on what then include in
	// this field, such as service region, type, tiers, etc...
	ServiceProviderSpec runtime.RawExtension `json:"serviceProviderSpec,omitempty"`
}

// ClusterBindingStatus stores status information about a service binding. It is
// updated by both the konnector and the service provider.
type ClusterBindingStatus struct {
	// lastHeartbeatTime is the last time the konnector updated the status.
	LastHeartbeatTime metav1.Time `json:"lastHeartbeatTime,omitempty"`

	// heartbeatInterval is the maximal interval between heartbeats that the
	// konnector promises to send. The service provider can assume that the
	// konnector is not unhealthy if it does not receive a heartbeat within
	// this time.
	HeartbeatInterval metav1.Duration `json:"heartbeatInterval,omitempty"`

	// konnectorVersion is the version of the konnector that is running on the
	// consumer cluster.
	KonnectorVersion string `json:"konnectorVersion,omitempty"`

	// conditions is a list of conditions that apply to the ClusterBinding. It is
	// updated by the konnector and the service provider.
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
