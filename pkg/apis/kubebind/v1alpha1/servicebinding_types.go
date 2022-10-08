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

	conditionsapi "github.com/kube-bind/kube-bind/pkg/apis/third_party/conditions/apis/conditions/v1alpha1"
)

// ServiceBinding binds an API service represented by a ServiceExport
// in a service provider cluster into a consumer cluster. This object lives in
// the consumer cluster.
//
// +crd
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:subresource:status
type ServiceBinding struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec specifies how an API service from a service provider should be bound in the
	// local consumer cluster.
	Spec ServiceBindingSepc `json:"spec"`

	// status contains reconciliation information for a service binding.
	Status ServiceBindingStatus `json:"status,omitempty"`
}

type ServiceBindingSepc struct {
	// kubeconfigSecretName is the secret ref that contains the kubeconfig of the service cluster.
	KubeconfigSecretRef ClusterSecretKeyRef `json:"kubeconfigSecretRef"`
}

type ServiceBindingStatus struct {
	// conditions is a list of conditions that apply to the ServiceBinding.
	//
	// +optional
	Conditions conditionsapi.Conditions `json:"conditions,omitempty"`
}

// ServiceBindingList is a list of ServiceBindings.
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ServiceBindingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []ServiceExport `json:"items"`
}

type LocalSecretKeyRef struct {
	// Name of the referent.
	// +required
	Name string `json:"name,omitempty" protobuf:"bytes,1,opt,name=name"`

	// The key of the secret to select from.  Must be a valid secret key.
	// +required
	Key string `json:"key" protobuf:"bytes,2,opt,name=key"`
}

type ClusterSecretKeyRef struct {
	LocalSecretKeyRef `json:",inline"`

	// Namespace of the referent.
	// +required
	Namespace string `json:"namespace,omitempty"`
}
