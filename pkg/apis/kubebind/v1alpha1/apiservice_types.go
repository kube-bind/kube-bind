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

	conditionsapi "github.com/kube-bind/kube-bind/pkg/apis/third_party/conditions/apis/conditions/v1alpha1"
)

// APIService is a non-CRUD resource that is returned by the server before
// authentication.
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type APIService struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec specifies how an API service from a service provider should be bound in the
	// local consumer cluster.
	Spec APIServiceSpec `json:"spec"`

	// status contains reconciliation information for a service binding.
	Status APIServiceStatus `json:"status,omitempty"`
}

type APIServiceSpec struct {
}

type APIServiceStatus struct {
	// conditions is a list of conditions that apply to the ServiceBinding.
	Conditions conditionsapi.Conditions `json:"conditions,omitempty"`
}

// APIServiceList is a list of APIServices.
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type APIServiceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []APIService `json:"items"`
}
