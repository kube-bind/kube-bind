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

const (
	// ServiceBindingConditionSecretValid is set when the secret is valid.
	ServiceBindingConditionSecretValid conditionsapi.ConditionType = "SecretValid"

	// ServiceBindingConditionInformersSynced is set when the informers can sync.
	ServiceBindingConditionInformersSynced conditionsapi.ConditionType = "InformersSynced"

	// ServiceBindingConditionHeartbeating is set when the ClusterBinding of the service provider
	// is successfully heartbeated.
	ServiceBindingConditionHeartbeating conditionsapi.ConditionType = "Heartbeating"

	// ServiceBindingConditionConnected means the ServiceBinding has been connected to a ServiceExport.
	ServiceBindingConditionConnected conditionsapi.ConditionType = "Connected"

	// ServiceBindingConditionSchemaInSync is set to true when the ServiceExport's
	// schema is applied to the consumer cluster.
	ServiceBindingConditionSchemaInSync conditionsapi.ConditionType = "SchemaInSync"
)

// ServiceBinding binds an API service represented by a ServiceExport
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
type ServiceBinding struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec specifies how an API service from a service provider should be bound in the
	// local consumer cluster.
	Spec ServiceBindingSpec `json:"spec"`

	// status contains reconciliation information for a service binding.
	Status ServiceBindingStatus `json:"status,omitempty"`
}

func (in *ServiceBinding) GetConditions() conditionsapi.Conditions {
	return in.Status.Conditions
}

func (in *ServiceBinding) SetConditions(conditions conditionsapi.Conditions) {
	in.Status.Conditions = conditions
}

type ServiceBindingSpec struct {
	// export is the name of the ServiceExport object in the service provider cluster.
	//
	// +required
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Export string `json:"export"`

	// kubeconfigSecretName is the secret ref that contains the kubeconfig of the service cluster.
	//
	// +required
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="kubeconfigSecretRef is immutable"
	KubeconfigSecretRef ClusterSecretKeyRef `json:"kubeconfigSecretRef"`
}

type ServiceBindingStatus struct {
	// providerPrettyName is the pretty name of the service provider cluster. This
	// can be shared among different ServiceBindings.
	ProviderPrettyName string `json:"providerPrettyName,omitempty"`

	// conditions is a list of conditions that apply to the ServiceBinding.
	Conditions conditionsapi.Conditions `json:"conditions,omitempty"`
}

// ServiceBindingList is a list of ServiceBindings.
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ServiceBindingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []ServiceBinding `json:"items"`
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
