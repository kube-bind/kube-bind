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
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//+genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:shortName=sb
// +kubebuilder:subresource:status

// ServiceExport is the object that represents the ClusterBinding.
type ServiceExport struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec represents the data in the newly created service binding export.
	Spec ServiceExportSepc `json:"spec"`

	// status contains reconciliation information for the service binding export.
	Status ServiceBindingExportStatus `json:"status,omitempty"`
}

type ServiceExportSepc struct {
	// permissionClaims make resources available in the ServiceExport virtual workspace that are not part
	// of the actual APIExport resources.
	//
	// PermissionClaims are optional and should be the least access necessary to complete the functions
	// that the service provider needs. Access is asked for on a GroupResource + identity basis.
	//
	// PermissionClaims must be accepted by the user's explicit acknowledgement. Hence, when claims
	// change, the respecting objects are not visible immediately.
	//
	// PermissionClaims overlapping with the ServiceExport resources are ignored.
	//
	// +optional
	// +listType=map
	// +listMapKey=group
	// +listMapKey=resource
	PermissionClaims []PermissionClaim `json:"permissionClaims,omitempty"`
}

type ServiceBindingExportStatus struct {
}

// PermissionClaim identifies an object by GR and identity hash.
// Its purpose is to determine the added permissions that a service provider may
// request and that a consumer may accept and allow the service provider access to.
type PermissionClaim struct {
	GroupResource `json:","`

	// identityHash is the identity for a given ServiceExport that the APIResourceSchema belongs to.
	// The hash can be found on ServiceExport and APIResourceSchema's status.
	// It will be empty for core types.
	// Note that one must look this up for a particular KCP instance.
	// +optional
	IdentityHash string `json:"identityHash,omitempty"`
}

func (p PermissionClaim) String() string {
	if p.IdentityHash == "" {
		return fmt.Sprintf("%s.%s", p.Resource, p.Group)
	}
	return fmt.Sprintf("%s.%s:%s", p.Resource, p.Group, p.IdentityHash)
}

func (p PermissionClaim) Equal(claim PermissionClaim) bool {
	return p.Group == claim.Group &&
		p.Resource == claim.Resource &&
		p.IdentityHash == claim.IdentityHash
}

// GroupResource identifies a resource.
type GroupResource struct {
	// group is the name of an API group.
	// For core groups this is the empty string '""'.
	//
	// +kubebuilder:validation:Pattern=`^(|[a-z0-9]([-a-z0-9]*[a-z0-9](\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*)?)$`
	// +optional
	Group string `json:"group,omitempty"`

	// resource is the name of the resource.
	// Note: it is worth noting that you can not ask for permissions for resource provided by a CRD
	// not provided by an service binding export.
	// +kubebuilder:validation:Pattern=`^[a-z][-a-z0-9]*[a-z0-9]$`
	// +required
	// +kubebuilder:validation:Required
	Resource string `json:"resource"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ServiceExportList is the objects list that represents the ServiceExport.
type ServiceExportList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []ServiceExport `json:"items"`
}
