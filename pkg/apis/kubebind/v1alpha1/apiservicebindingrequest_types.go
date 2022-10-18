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
	// APIServiceBindingRequestConditionExportsReady is set to true when the
	// corresponding APIServiceExport is ready.
	APIServiceBindingRequestConditionExportsReady conditionsapi.ConditionType = "ExportsReady"
)

// APIServiceBindingRequest is represents a request session of kubectl-bind-apiservice.
//
// The service provider can prune these objects after some time.
//
// +crd
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:scope=Namespaced,categories=kube-bindings
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=`.status.conditions[?(@.type=="Ready")].status`,priority=0
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=`.metadata.creationTimestamp`,priority=0
type APIServiceBindingRequest struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec specifies how an API service from a service provider should be bound in the
	// local consumer cluster.
	//
	// +required
	// +kubebuilder:validation:Required
	Spec APIServiceBindingRequestSpec `json:"spec"`

	// status contains reconciliation information for a service binding.
	Status APIServiceBindingRequestStatus `json:"status,omitempty"`
}

// APIServiceBindingRequestResponse is like APIServiceBindingRequest but without
// ObjectMeta, to avoid unwanted metadata fields being sent in the response.
type APIServiceBindingRequestResponse struct {
	metav1.TypeMeta `json:",inline"`

	// spec specifies how an API service from a service provider should be bound in the
	// local consumer cluster.
	//
	// +required
	// +kubebuilder:validation:Required
	Spec APIServiceBindingRequestSpec `json:"spec"`

	// status contains reconciliation information for a service binding.
	Status APIServiceBindingRequestStatus `json:"status,omitempty"`
}

func (in *APIServiceBindingRequest) GetConditions() conditionsapi.Conditions {
	return in.Status.Conditions
}

func (in *APIServiceBindingRequest) SetConditions(conditions conditionsapi.Conditions) {
	in.Status.Conditions = conditions
}

type APIServiceBindingRequestSpec struct {
	// parameters holds service provider specific parameters for this binding
	// request.
	//
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="parameters are immutable"
	Parameters *runtime.RawExtension `json:"parameters,omitempty"`

	// resources is a list of resources that should be exported.
	//
	// +required
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="resources are immutable"
	Resources []APIServiceBindingRequestResource `json:"resources"`
}

type APIServiceBindingRequestResource struct {
	GroupResource `json:",inline"`

	// versions is a list of versions that should be exported. If this is empty
	// a sensible default is chosen by the service provider.
	Versions []string `json:"versions,omitempty"`
}

type APIServiceBindingRequestPhase string

const (
	// APIServiceBindingRequestPhasePending indicates that the service binding
	// is in progress.
	APIServiceBindingRequestPhasePending APIServiceBindingRequestPhase = "Pending"
	// APIServiceBindingRequestPhaseFailed indicates that the service binding
	// has failed. It will not resume.
	APIServiceBindingRequestPhaseFailed APIServiceBindingRequestPhase = "Failed"
	// APIServiceBindingRequestPhaseSucceeded indicates that the service binding
	// has succeeded. The corresponding APIServiceExport have been created and
	// are ready.
	APIServiceBindingRequestPhaseSucceeded APIServiceBindingRequestPhase = "Succeeded"
)

type APIServiceBindingRequestStatus struct {
	// phase is the current phase of the binding request. It starts in Pending
	// and transitions to Succeeded or Failed. See the condition for detailed
	// information.
	//
	// +optional
	// +kubebuilder:validation:Optional
	// +kubebuilder:Default=Pending
	// +kubebuilder:validation:Enum=Pending;Failed;Succeeded
	Phase APIServiceBindingRequestPhase `json:"phase,omitempty"`

	// export is the name of the APIServiceExport object that has been created
	// for this binding request when phase is Succeeded.
	Export string `json:"export,omitempty"`

	// conditions is a list of conditions that apply to the ClusterBinding. It is
	// updated by the konnector and the service provider.
	Conditions conditionsapi.Conditions `json:"conditions,omitempty"`
}

// APIServiceBindingRequestList is the list of APIServiceBindingRequest.
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type APIServiceBindingRequestList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []APIServiceBindingRequest `json:"items"`
}
