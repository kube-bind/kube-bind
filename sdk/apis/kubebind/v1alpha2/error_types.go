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
)

// Error represents a structured error response from the kube-bind backend
// +kubebuilder:object:root=true
type Error struct {
	metav1.TypeMeta `json:",inline"`
	// Message is the human-readable error message
	Message string `json:"message"`
	// Code is the error code that can be used for programmatic handling
	Code string `json:"code"`
	// Details provides additional context about the error
	Details string `json:"details,omitempty"`
}

// Common error codes
const (
	// ErrorCodeAuthenticationFailed indicates authentication failure
	ErrorCodeAuthenticationFailed = "AUTHENTICATION_FAILED"
	// ErrorCodeAuthorizationFailed indicates authorization failure  
	ErrorCodeAuthorizationFailed = "AUTHORIZATION_FAILED"
	// ErrorCodeResourceNotFound indicates a requested resource was not found
	ErrorCodeResourceNotFound = "RESOURCE_NOT_FOUND"
	// ErrorCodeInternalError indicates an internal server error
	ErrorCodeInternalError = "INTERNAL_ERROR"
	// ErrorCodeBadRequest indicates a malformed request
	ErrorCodeBadRequest = "BAD_REQUEST"
	// ErrorCodeClusterConnectionFailed indicates failure to connect to cluster
	ErrorCodeClusterConnectionFailed = "CLUSTER_CONNECTION_FAILED"
)

// NewError creates a new Error with the specified code, message and optional details
func NewError(code, message, details string) *Error {
	return &Error{
		TypeMeta: metav1.TypeMeta{
			APIVersion: SchemeGroupVersion.String(),
			Kind:       "Error",
		},
		Code:    code,
		Message: message,
		Details: details,
	}
}

// Error implements the error interface
func (e *Error) Error() string {
	return e.Message
}