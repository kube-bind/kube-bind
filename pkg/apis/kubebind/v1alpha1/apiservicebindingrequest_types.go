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
)

// APIServiceBindingRequest is posted to the URL in spec.url to establish
// a binding on the service provider side. The service provider will create
// a ClusterBinding, APIServiceExports and return a kubeconfig to the namespace
// (with the namespace set in the current context) they are in.
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type APIServiceBindingRequest struct {
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

type APIServiceBindingRequestSpec struct {
	// url is the URL this request should be posted.
	//
	// +required
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Fomat=url
	// +kubebuilder:validation:MinLength=1
	URL string `json:"url"`

	// bindings specifies the list of bindings to create. It is expected that,
	// on success of the request, the service provider will create equally named
	// APIServiceExports. The client is not expected to bind to any other exports
	// than those listed here.
	//
	// +required
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	Bindings []APIServiceBindingRequestBinding `json:"bindings"`
}

type APIServiceBindingRequestBinding struct {
	// Name is the name of the ServiceBinding to create.
	//
	// +required
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// parameters is a service-provider specific object that contains the parameters
	// to create the given ServiceBinding.
	Parameters runtime.RawExtension `json:"parameters,omitempty"`
}

type APIServiceBindingRequestStatus struct {
	// kubeConfig is the kubeconfig to access the service provider cluster. It is
	// expected that this kubeconfig in its current context points to the namespace
	// that holds ServiceExports and the ClusterBinding object.
	//
	// TODO: think about how this would look like with a bound service account token.
	KubeConfig []byte `json:"kubeConfig,omitempty"`
}
