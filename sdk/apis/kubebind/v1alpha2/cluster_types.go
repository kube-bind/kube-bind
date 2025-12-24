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

const (
	// ClusterConditionHealthy is set when the cluster is healthy.
	ClusterConditionHealthy = "Healthy"
)

const (
	// DefaultClusterName is the name of the default cluster.
	DefaultClusterName = "default"
)

// Cluster represents a cluster, managed by kube-bind.
//
// +crd
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:scope=Cluster,categories=kube-bind
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=`.status.conditions[?(@.type=="Ready")].status`,priority=0
type Cluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	// spec represents the data in the newly created ClusterBinding.
	// +required
	// +kubebuilder:validation:Required
	Spec ClusterSpec `json:"spec"`

	// status contains reconciliation information for the service binding.
	Status ClusterStatus `json:"status"`
}

func (in *Cluster) GetConditions() conditionsapi.Conditions {
	return in.Status.Conditions
}

func (in *Cluster) SetConditions(conditions conditionsapi.Conditions) {
	in.Status.Conditions = conditions
}

// ClusterSpec represents the data in the newly created Cluster.
type ClusterSpec struct {
}

// ClusterStatus stores status information about a service binding. It is
// updated by both the konnector and the service provider.
type ClusterStatus struct {
	// conditions is a list of conditions that apply to the ClusterBinding. It is
	// updated by the konnector and the service provider.
	Conditions conditionsapi.Conditions `json:"conditions,omitempty"`
}

// ClusterIdentity contains information that uniquely identifies the cluster.
type ClusterIdentity struct {
	// Identity is the unique identifier of the cluster.
	Identity string `json:"identity,omitempty"`
}

// ClusterList is the objects list that represents the Cluster.
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Cluster `json:"items"`
}
