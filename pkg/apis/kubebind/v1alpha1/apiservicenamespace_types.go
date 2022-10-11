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
)

const (
	APIServiceNamespaceAnnotationKey = "kube-bind.io/api-service-namespace"
)

// APIServiceNamespace defines how consumer namespaces map to service namespaces.
// These objects are created by the konnector, and a service namespace is then
// created by the service provider.
//
// The name of the APIServiceNamespace equals the namespace name in the consumer
// cluster.
//
// +crd
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:scope=Namespaced,categories=kube-bindings
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Namespace",type="string",JSONPath=`.status.namespace`,priority=0
type APIServiceNamespace struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec specifies a service namespace.
	Spec APIServiceNamespaceSpec `json:"spec"`

	// status contains reconciliation information for a service namespace
	Status APIServiceNamespaceStatus `json:"status,omitempty"`
}

type APIServiceNamespaceSpec struct {
}

type APIServiceNamespaceStatus struct {
	// namespace is the service provider namespace name that will be bound to the
	// consumer namespace named like this object.
	Namespace string `json:"namespace,omitempty"`
}

// APIServiceNamespaceList is the list of ServiceNamespaces.
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type APIServiceNamespaceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []APIServiceNamespace `json:"items"`
}
