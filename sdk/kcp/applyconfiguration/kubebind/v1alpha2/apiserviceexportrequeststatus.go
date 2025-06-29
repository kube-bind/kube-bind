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

package v1alpha2

import (
	v1alpha2 "github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2"
	v1alpha1 "github.com/kube-bind/kube-bind/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
)

// APIServiceExportRequestStatusApplyConfiguration represents a declarative configuration of the APIServiceExportRequestStatus type for use
// with apply.
type APIServiceExportRequestStatusApplyConfiguration struct {
	Phase           *v1alpha2.APIServiceExportRequestPhase `json:"phase,omitempty"`
	TerminalMessage *string                                `json:"terminalMessage,omitempty"`
	Conditions      *v1alpha1.Conditions                   `json:"conditions,omitempty"`
}

// APIServiceExportRequestStatusApplyConfiguration constructs a declarative configuration of the APIServiceExportRequestStatus type for use with
// apply.
func APIServiceExportRequestStatus() *APIServiceExportRequestStatusApplyConfiguration {
	return &APIServiceExportRequestStatusApplyConfiguration{}
}

// WithPhase sets the Phase field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the Phase field is set to the value of the last call.
func (b *APIServiceExportRequestStatusApplyConfiguration) WithPhase(value v1alpha2.APIServiceExportRequestPhase) *APIServiceExportRequestStatusApplyConfiguration {
	b.Phase = &value
	return b
}

// WithTerminalMessage sets the TerminalMessage field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the TerminalMessage field is set to the value of the last call.
func (b *APIServiceExportRequestStatusApplyConfiguration) WithTerminalMessage(value string) *APIServiceExportRequestStatusApplyConfiguration {
	b.TerminalMessage = &value
	return b
}

// WithConditions sets the Conditions field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the Conditions field is set to the value of the last call.
func (b *APIServiceExportRequestStatusApplyConfiguration) WithConditions(value v1alpha1.Conditions) *APIServiceExportRequestStatusApplyConfiguration {
	b.Conditions = &value
	return b
}
