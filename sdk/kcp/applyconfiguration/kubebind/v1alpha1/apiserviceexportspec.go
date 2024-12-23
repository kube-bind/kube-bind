/*
Copyright The Kube Bind Authors.

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

// Code generated by applyconfiguration-gen-v0.31. DO NOT EDIT.

package v1alpha1

import (
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	kubebindv1alpha1 "github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha1"
)

// APIServiceExportSpecApplyConfiguration represents a declarative configuration of the APIServiceExportSpec type for use
// with apply.
type APIServiceExportSpecApplyConfiguration struct {
	APIServiceExportCRDSpecApplyConfiguration `json:",inline"`
	InformerScope                             *kubebindv1alpha1.Scope     `json:"informerScope,omitempty"`
	ClusterScopedIsolation                    *kubebindv1alpha1.Isolation `json:"clusterScopedIsolation,omitempty"`
}

// APIServiceExportSpecApplyConfiguration constructs a declarative configuration of the APIServiceExportSpec type for use with
// apply.
func APIServiceExportSpec() *APIServiceExportSpecApplyConfiguration {
	return &APIServiceExportSpecApplyConfiguration{}
}

// WithGroup sets the Group field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the Group field is set to the value of the last call.
func (b *APIServiceExportSpecApplyConfiguration) WithGroup(value string) *APIServiceExportSpecApplyConfiguration {
	b.Group = &value
	return b
}

// WithNames sets the Names field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the Names field is set to the value of the last call.
func (b *APIServiceExportSpecApplyConfiguration) WithNames(value v1.CustomResourceDefinitionNames) *APIServiceExportSpecApplyConfiguration {
	b.Names = &value
	return b
}

// WithScope sets the Scope field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the Scope field is set to the value of the last call.
func (b *APIServiceExportSpecApplyConfiguration) WithScope(value v1.ResourceScope) *APIServiceExportSpecApplyConfiguration {
	b.Scope = &value
	return b
}

// WithVersions adds the given value to the Versions field in the declarative configuration
// and returns the receiver, so that objects can be build by chaining "With" function invocations.
// If called multiple times, values provided by each call will be appended to the Versions field.
func (b *APIServiceExportSpecApplyConfiguration) WithVersions(values ...*APIServiceExportVersionApplyConfiguration) *APIServiceExportSpecApplyConfiguration {
	for i := range values {
		if values[i] == nil {
			panic("nil value passed to WithVersions")
		}
		b.Versions = append(b.Versions, *values[i])
	}
	return b
}

// WithInformerScope sets the InformerScope field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the InformerScope field is set to the value of the last call.
func (b *APIServiceExportSpecApplyConfiguration) WithInformerScope(value kubebindv1alpha1.Scope) *APIServiceExportSpecApplyConfiguration {
	b.InformerScope = &value
	return b
}

// WithClusterScopedIsolation sets the ClusterScopedIsolation field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the ClusterScopedIsolation field is set to the value of the last call.
func (b *APIServiceExportSpecApplyConfiguration) WithClusterScopedIsolation(value kubebindv1alpha1.Isolation) *APIServiceExportSpecApplyConfiguration {
	b.ClusterScopedIsolation = &value
	return b
}
