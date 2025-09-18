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
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// InternalAPI describes an API to be imported from some schemes and generated OpenAPI V2 definitions.
type InternalAPI struct {
	Names                apiextensionsv1.CustomResourceDefinitionNames `json:"names"`
	GroupVersionResource GroupVersionResource                          `json:"groupVersionResource"`
	Instance             runtime.Object                                `json:"instance"`
	ResourceScope        apiextensionsv1.ResourceScope                 `json:"resourceScope"`
	HasStatus            bool                                          `json:"hasStatus"`
}

// ClaimableAPIs is a list of APIs that can be claimed by a user.
type ClaimableAPIs []InternalAPI

// ClaimableAPIsData is a list of APIs that can be claimed by a user.
var ClaimableAPIsData = ClaimableAPIs{
	{
		Names: apiextensionsv1.CustomResourceDefinitionNames{
			Plural:   "configmaps",
			Singular: "configmap",
			Kind:     "ConfigMap",
		},
		GroupVersionResource: GroupVersionResource{
			Group:    "",
			Version:  "v1",
			Resource: "configmaps",
		},
		Instance:      &corev1.ConfigMap{},
		ResourceScope: apiextensionsv1.NamespaceScoped,
	},
	{
		Names: apiextensionsv1.CustomResourceDefinitionNames{
			Plural:   "secrets",
			Singular: "secret",
			Kind:     "Secret",
		},
		GroupVersionResource: GroupVersionResource{
			Group:    "",
			Version:  "v1",
			Resource: "secrets",
		},
		Instance:      &corev1.Secret{},
		ResourceScope: apiextensionsv1.NamespaceScoped,
	},
	{
		Names: apiextensionsv1.CustomResourceDefinitionNames{
			Plural:   "serviceaccounts",
			Singular: "serviceaccount",
			Kind:     "ServiceAccount",
		},
		GroupVersionResource: GroupVersionResource{
			Group:    "",
			Version:  "v1",
			Resource: "serviceaccounts",
		},
		Instance:      &corev1.ServiceAccount{},
		ResourceScope: apiextensionsv1.NamespaceScoped,
	},
}

func ResolveClaimableAPI(claim PermissionClaim) (GroupVersionResource, error) {
	for _, api := range ClaimableAPIsData {
		if api.Names.Plural == claim.Resource && api.GroupVersionResource.Group == claim.Group {
			return api.GroupVersionResource, nil
		}
	}
	return GroupVersionResource{}, fmt.Errorf("no matching API found")
}
