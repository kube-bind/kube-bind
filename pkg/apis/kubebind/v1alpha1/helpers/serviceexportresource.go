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

package helpers

import (
	"fmt"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	kubebindv1alpha1 "github.com/kube-bind/kube-bind/pkg/apis/kubebind/v1alpha1"
)

// ServiceExportResourceToCRD converts a ServiceExportResource to a CRD.
func ServiceExportResourceToCRD(resource *kubebindv1alpha1.ServiceExportResource) (*apiextensionsv1.CustomResourceDefinition, error) {
	crd := &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: resource.Name,
		},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Group: resource.Spec.Group,
			Names: resource.Spec.Names,
			Scope: resource.Spec.Scope,
		},
	}

	for i := range resource.Spec.Versions {
		resourceVersion := resource.Spec.Versions[i]

		crdVersion := apiextensionsv1.CustomResourceDefinitionVersion{
			Name:                     resourceVersion.Name,
			Served:                   resourceVersion.Served,
			Storage:                  resourceVersion.Storage,
			Deprecated:               resourceVersion.Deprecated,
			DeprecationWarning:       resourceVersion.DeprecationWarning,
			AdditionalPrinterColumns: resourceVersion.AdditionalPrinterColumns,
		}

		if len(resourceVersion.Schema.OpenAPIV3Schema.Raw) > 0 {
			var schema apiextensionsv1.JSONSchemaProps
			if err := yaml.Unmarshal(resourceVersion.Schema.OpenAPIV3Schema.Raw, &schema); err != nil {
				return nil, fmt.Errorf("failed to unmarshal schema for version %q: %w", resourceVersion.Name, err)
			}
			crdVersion.Schema = &apiextensionsv1.CustomResourceValidation{
				OpenAPIV3Schema: &schema,
			}
		}

		crdVersion.Subresources = &resourceVersion.Subresources

		crd.Spec.Versions = append(crd.Spec.Versions, crdVersion)
	}

	return crd, nil
}
