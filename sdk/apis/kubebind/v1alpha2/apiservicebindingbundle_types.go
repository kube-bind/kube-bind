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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	conditionsapi "github.com/kube-bind/kube-bind/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
)

// APIServiceBindingBundle automatically binds multiple API services represented by
// APIServiceExports in a service provider cluster into a consumer cluster. This object lives in consumer clusters,
// and pulls in all API services defined in the provider clusters, based on the kubeconfig secret provided.
//
// +crd
// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:scope=Cluster,categories=kube-bindings,shortName=sbb
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
// +kubebuilder:printcolumn:name="Provider",type="string",JSONPath=`.status.providerPrettyName`,priority=0
// +kubebuilder:printcolumn:name="Resources",type="string",JSONPath=`.metadata.annotations.kube-bind\.io/resources`,priority=1
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=`.status.conditions[?(@.type=="Ready")].status`,priority=0
// +kubebuilder:printcolumn:name="Message",type="string",JSONPath=`.status.conditions[?(@.type=="Ready")].message`,priority=0
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=`.metadata.creationTimestamp`,priority=0
type APIServiceBindingBundle struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	// spec specifies how an API service from a service provider should be bound in the
	// local consumer cluster.
	Spec APIServiceBindingBundleSpec `json:"spec"`

	// status contains reconciliation information for a service binding.
	Status APIServiceBindingBundleStatus `json:"status"`
}

func (in *APIServiceBindingBundle) GetConditions() conditionsapi.Conditions {
	return in.Status.Conditions
}

func (in *APIServiceBindingBundle) SetConditions(conditions conditionsapi.Conditions) {
	in.Status.Conditions = conditions
}

type APIServiceBindingBundleSpec struct {
	// kubeconfigSecretName is the secret ref that contains the kubeconfig of the service cluster.
	//
	// +required
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="kubeconfigSecretRef is immutable"
	KubeconfigSecretRef ClusterSecretKeyRef `json:"kubeconfigSecretRef"`
}

type APIServiceBindingBundleStatus struct {
	// conditions is a list of conditions that apply to the APIServiceBindingBundle.
	Conditions conditionsapi.Conditions `json:"conditions,omitempty"`
}

// APIServiceBindingBundleList is a list of APIServiceBindingBundles.
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type APIServiceBindingBundleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []APIServiceBindingBundle `json:"items"`
}
