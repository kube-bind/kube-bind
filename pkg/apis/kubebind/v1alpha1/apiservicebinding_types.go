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

const (
	// APIServiceBindingConditionSecretValid is set when the secret is valid.
	APIServiceBindingConditionSecretValid conditionsapi.ConditionType = "SecretValid"

	// APIServiceBindingConditionInformersSynced is set when the informers can sync.
	APIServiceBindingConditionInformersSynced conditionsapi.ConditionType = "InformersSynced"

	// APIServiceBindingConditionHeartbeating is set when the ClusterBinding of the service provider
	// is successfully heartbeated.
	APIServiceBindingConditionHeartbeating conditionsapi.ConditionType = "Heartbeating"

	// APIServiceBindingConditionConnected means the APIServiceBinding has been connected to a APIServiceExport.
	APIServiceBindingConditionConnected conditionsapi.ConditionType = "Connected"

	// APIServiceBindingConditionSchemaInSync is set to true when the APIServiceExport's
	// schema is applied to the consumer cluster.
	APIServiceBindingConditionSchemaInSync conditionsapi.ConditionType = "SchemaInSync"

	// DownstreamFinalizer is put on downstream objects to block their deletion until
	// the upstream object has been deleted.
	DownstreamFinalizer = "kubebind.io/syncer"
)

// APIServiceBinding binds an API service represented by a APIServiceExport
// in a service provider cluster into a consumer cluster. This object lives in
// the consumer cluster.
//
// +crd
// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:scope=Cluster,categories=kube-bindings,shortName=sb
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Provider",type="string",JSONPath=`.status.providerPrettyName`,priority=0
// +kubebuilder:printcolumn:name="Resources",type="string",JSONPath=`.metadata.annotations.kube-bind\.io/resources`,priority=1
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=`.status.conditions[?(@.type=="Ready")].status`,priority=0
// +kubebuilder:printcolumn:name="Message",type="string",JSONPath=`.status.conditions[?(@.type=="Ready")].message`,priority=0
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=`.metadata.creationTimestamp`,priority=0
type APIServiceBinding struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec specifies how an API service from a service provider should be bound in the
	// local consumer cluster.
	Spec APIServiceBindingSpec `json:"spec"`

	// status contains reconciliation information for a service binding.
	Status APIServiceBindingStatus `json:"status,omitempty"`
}

func (in *APIServiceBinding) GetConditions() conditionsapi.Conditions {
	return in.Status.Conditions
}

func (in *APIServiceBinding) SetConditions(conditions conditionsapi.Conditions) {
	in.Status.Conditions = conditions
}

type APIServiceBindingSpec struct {
	// kubeconfigSecretName is the secret ref that contains the kubeconfig of the service cluster.
	//
	// +required
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="kubeconfigSecretRef is immutable"
	KubeconfigSecretRef ClusterSecretKeyRef `json:"kubeconfigSecretRef"`

	// permissionClaims records decisions about permission claims requested by the API service provider.
	// Individual claims can be accepted or rejected. If accepted, the API service provider gets the
	// requested access to the specified resources in this workspace. Access is granted per
	// GroupResource, identity, and other properties.
	//
	// +optional
	PermissionClaims []AcceptablePermissionClaim `json:"permissionClaims,omitempty"`
}

type AcceptablePermissionClaim struct {
	PermissionClaim `json:",inline"`

	// state indicates if the claim is accepted or rejected.

	// +required
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=Accepted;Rejected
	State AcceptablePermissionClaimState `json:"state"`
}

type AcceptablePermissionClaimState string

const (
	ClaimAccepted AcceptablePermissionClaimState = "Accepted"
	ClaimRejected AcceptablePermissionClaimState = "Rejected"
)

// PermissionClaim identifies an object by GR and identity hash.
// Its purpose is to determine the added permissions that a service provider may
// request and that a consumer may accept and allow the service provider access to.
//
// TODO fix validation
// kubebuilder:validation:XValidation:rule="(has(self.all) && self.all) != (has(self.resourceSelector) && size(self.resourceSelector) > 0)",message="either \"all\" or \"resourceSelector\" must be set"
type PermissionClaim struct {
	GroupResource `json:","`

	Version string `json:"version"`

	// all claims all resources for the given group/resource.
	// This is mutually exclusive with resourceSelector.
	// +optional
	All bool `json:"all,omitempty"`

	Verbs ClaimVerbs `json:"verbs"`

	// resourceSelector is a list of claimed resource selectors.
	//
	// +optional
	ResourceSelector []ResourceSelector `json:"resourceSelector,omitempty"`
}

// +kubebuilder:validation:XValidation:rule="has(self.__namespace__) || has(self.name)",message="at least one field must be set"
type ResourceSelector struct {
	// name of an object within a claimed group/resource.
	// It matches the metadata.name field of the underlying object.
	// If namespace is unset, all objects matching that name will be claimed.
	//
	// +optional
	// +kubebuilder:validation:Pattern="^([a-z0-9][-a-z0-9_.]*)?[a-z0-9]$"
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name,omitempty"`

	// namespace containing the named object. Matches metadata.namespace field.
	// If "name" is unset, all objects from the namespace are being claimed.
	//
	// +optional
	// +kubebuilder:validation:MinLength=1
	Namespace string `json:"namespace,omitempty"`

	//
	// WARNING: If adding new fields, add them to the XValidation check!
	//
}

type ClaimVerbs struct {
	Provider []Verb `json:"provider"`
	Consumer []Verb `json:"consumer"`
}

type Verb string

type APIServiceBindingStatus struct {
	// providerPrettyName is the pretty name of the service provider cluster. This
	// can be shared among different APIServiceBindings.
	ProviderPrettyName string `json:"providerPrettyName,omitempty"`

	// conditions is a list of conditions that apply to the APIServiceBinding.
	Conditions conditionsapi.Conditions `json:"conditions,omitempty"`
}

// APIServiceBindingList is a list of APIServiceBindings.
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type APIServiceBindingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []APIServiceBinding `json:"items"`
}

type LocalSecretKeyRef struct {
	// Name of the referent.
	//
	// +required
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// The key of the secret to select from.  Must be "kubeconfig".
	//
	// +required
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=kubeconfig
	Key string `json:"key"`
}

type ClusterSecretKeyRef struct {
	LocalSecretKeyRef `json:",inline"`

	// Namespace of the referent.
	//
	// +required
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Namespace string `json:"namespace"`
}
