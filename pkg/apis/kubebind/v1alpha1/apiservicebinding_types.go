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

// PermissionClaim selects objects of a GVR that a service provider may
// request and that a consumer may accept and allow the service provider access to.
//
// +kubebuilder:validation:XValidation:rule="!(has(self.autoDonate) && self.autoDonate && has(self.autoAdopt) && self.autoAdopt)",message="donate and adopt are mutually exclusive"
type PermissionClaim struct {
	GroupResource `json:","`

	// +kubebuilder:validation:Required
	Version string `json:"version"`

	// Selector selects which resources are affected by this claim.
	// +optional
	Selector *ResourceSelector `json:"selector,omitempty"`

	// Required indicates whether the APIServiceBinding will work if this claim is not accepted.
	Required bool `json:"required"`

	// +optional
	// +kubebuilder:default={}
	Read *ReadOptions `json:"read,omitempty"`

	// only for owner Provider
	//
	// Create determines whether the kube-bind konnector will sync matching objects from the
	// provider side down to the consumer cluster.
	//
	// +optional
	Create *CreateOptions `json:"create,omitempty"`

	// AutoAdopt set to true means that objects created by the consumer are adopted by the provider.
	// i.e. the provider will become the owner.
	//
	// +optional
	AutoAdopt bool `json:"autoAdopt,omitempty"`

	// AutoDonate set to true means that a newly created object by the provider is immediately owned by the consumer.
	// If false, the object stays in ownership of the provider.
	//
	// +optional
	AutoDonate bool `json:"autoDonate,omitempty"`

	// onConflict determines how the conflicts between objects on the consumer side
	// will be resolved.
	//
	// +optional
	OnConflict *OnConflictOptions `json:"onConflict,omitempty"`

	// Update lists a number of claimed permissions for the provider.
	// "field" and "preserving" are mutually exclusive.
	//
	// +optional
	Update *UpdateOptions `json:"update,omitempty"`
}

type ReadOptions struct {
	// Labels is a list of claimed label key wildcard patterns
	// that are synchronized from the consumer side to the provider on
	// objects that are owned by the consumer
	//
	// +optional
	Labels []Matcher `json:"labels,omitempty"`

	// labelsOnProviderOwnedObjects is a list of claimed label key wildcard
	// patterns that are synchronized from the consumer side
	// to the provider on objects owned by the provider.
	//
	// +optional
	LabelsOnProviderOwnedObjects []Matcher `json:"labelsOnProviderOwnedObjects,omitempty"`

	// Annotations is a list of claimed annotation key wildcard patterns
	// that are synchronized from the consumer side to the provider on
	// objects that are owned by the consumer
	//
	// +optional
	Annotations []Matcher `json:"annotations,omitempty"`

	// OverrideAnnotations is a list of claimed annotation key wildcard
	// patterns that are synchronized from the consumer side
	// to the provider on objects owned by the provider.
	//
	// +optional
	OverrideAnnotations []Matcher `json:"overrideAnnotations,omitempty"`
}

type Matcher struct {
	// +optional
	Pattern string `json:"pattern,omitempty"`
}

type OnConflictOptions struct {
	// RecreateWhenConsumerSideDeleted set to true (the default) means the provider will recreate the object
	// in case the object is missing on the consumer side, but has been synchronized before.
	//
	// If set to false, deleted provider-owned objects get deleted on the provider side as well.
	//
	// Even if the consumer mistakenly or intentionally
	// deletes the object, the provider will recreate it. If the field is set as false,
	// the provider will not recreate the object in case the object is deleted on the RecreateWhenConsumerSideDeleted
	// side.
	//
	// +kubebuilder:default:=true
	RecreateWhenConsumerSideDeleted bool `json:"recreateWhenConsumerSideDeleted"`
}

type CreateOptions struct {
	// ReplaceExisting means that an existing object owned by the consumer will be replaced by the provider object.
	//
	// If set to false, and a conflicting consumer object exists, it is not touched.
	// +optional
	ReplaceExisting bool `json:"replaceExisting,omitempty"`
}

type UpdateOptions struct {
	// Fields are a list of JSON Paths describing which parts of an object the provider wants to control.
	//
	// This field is ignored if the owner in the claim selector is set to "Provider".
	Fields []string `json:"fields,omitempty"`

	// Preserving is a list of JSON Paths describing which parts of an object owned by the provider the consumer keeps controlling.
	//
	// This field is ignored if the owner in the claim selector is set to "Consumer".
	Preserving []string `json:"preservings,omitempty"`

	// AlwaysRecreate, when true will delete the old object and create new ones
	// instead of updating. Useful for immutable objects.
	//
	// This does not apply to metadata field updates.
	AlwaysRecreate bool `json:"alwaysRecreate,omitempty"`

	// Labels is a list of claimed label keys or label wildcard patterns that are synchronized from the provider to the consumer for objects owned by the provider.
	//
	// By default, no labels are synced.
	//
	// +optional
	Labels []Matcher `json:"labels,omitempty"`

	// OverrideLabels is a list of claiemd label key wildcard patterns that are synchronized from the provider to the consumer for objects owned by the consumer.
	//
	// By default, no labels are synced.
	//
	// +optional
	OverrideLabels []Matcher `json:"overrideLabels,omitempty"`

	// Annotations is a list of claimed annotation keys or annotation wildcard patterns that are synchronized from the provider to the consumer for objects owned by the provider.
	//
	// By default, no annotations are synced.
	//
	// +optional
	Annotations []Matcher `json:"annotations,omitempty"`

	// OverrideAnnotations is a list of claiemd annotation key wildcard patterns that are synchronized from the provider to the consumer for objects owned by the consumer.
	//
	// By default, no annotations are synced.
	//
	// +optional
	OverrideAnnotations []Matcher `json:"overrideAnnotations,omitempty"`
}

type ResourceSelector struct {
	// Names is a list of specific resource names to select.
	// Names matches the metadata.name field of the underlying object.
	// An entry of "*" anywhere in the list means all object names of the group/resource within the "namespaces" field are claimed.
	// Wildcard entries other than "*" and regular expressions are currently unsupported.
	//
	// +kubebuilder:validation:XValidation:rule="self.all(n, n.matches('^[A-z]*|[*]$'))",message="only names or * are allowed"
	// +kubebuilder:default:={"*"}
	// +optional
	Names []string `json:"names,omitempty"`

	// Namespaces represents namespaces where an object of the given group/resoruce may be managed.
	// Namespaces matches against the metadata.namespace field. A value of "*" matches namespaced objects across all
	// namespaces. If namespaces is not set (an empty list), matches cluster-scoped resources.
	// If the "names" field is unset, all objects of the group/resource within the listed namespaces (or cluster) will be claimed.
	//
	// +kubebuilder:validation:XValidation:rule="self.all(n, n.matches('^[A-z]*|[*]$'))",message="only names or * are allowed"
	// +kubebuilder:default:={"*"}
	// +optional
	Namespaces []string `json:"namespaces,omitempty"`

	// LabelSelectors is a list of label selectors matching selected resources. label selectors follow the same rules as kubernetes label selectors,
	// see https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/.
	LabelSelectors []map[string]string `json:"labelSelectors,omitempty"`

	// FieldSelectors is a list of field selectors matching selected resources,
	// see https://kubernetes.io/docs/concepts/overview/working-with-objects/field-selectors/.
	FieldSelectors []string `json:"fieldSelectors,omitempty"`

	// +kubebuilder:validation:Enum=Provider;Consumer
	// +optional
	Owner Owner `json:"owner,omitempty"`
}

type Owner string

// Provider means that the owner of the resource is the Provider.
const Provider Owner = "Provider"

// Consumer means that the owner of the resource is the Consumer.
const Consumer Owner = "Consumer"

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
