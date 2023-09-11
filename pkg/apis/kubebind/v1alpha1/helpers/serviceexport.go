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
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math/big"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime2 "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/yaml"

	kubebindv1alpha1 "github.com/kube-bind/kube-bind/pkg/apis/kubebind/v1alpha1"
)

// ServiceExportToCRD converts a APIServiceExport to a CRD.
func ServiceExportToCRD(export *kubebindv1alpha1.APIServiceExport) (*apiextensionsv1.CustomResourceDefinition, error) {
	crd := &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: export.Name,
		},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Group: export.Spec.Group,
			Names: export.Spec.Names,
			Scope: export.Spec.Scope,
		},
	}

	for i := range export.Spec.Versions {
		resourceVersion := export.Spec.Versions[i]

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

// CRDToServiceExport converts a CRD to a APIServiceExport.
func CRDToServiceExport(crd *apiextensionsv1.CustomResourceDefinition) (*kubebindv1alpha1.APIServiceExportCRDSpec, error) {
	spec := &kubebindv1alpha1.APIServiceExportCRDSpec{
		Group: crd.Spec.Group,
		Names: crd.Spec.Names,
		Scope: crd.Spec.Scope,
	}

	onlyFirstServingVersion := crd.Spec.Conversion != nil && crd.Spec.Conversion.Strategy == apiextensionsv1.WebhookConverter
	// TODO: come up with an API to select versions
	for i := range crd.Spec.Versions {
		crdVersion := crd.Spec.Versions[i]

		// skip non-served versions
		if !crdVersion.Served {
			continue
		}
		// TODO
		if onlyFirstServingVersion && !crdVersion.Storage {
			continue
		}

		apiResourceVersion := kubebindv1alpha1.APIServiceExportVersion{
			Name:                     crdVersion.Name,
			Served:                   crdVersion.Served,
			Storage:                  crdVersion.Storage,
			Deprecated:               crdVersion.Deprecated,
			DeprecationWarning:       crdVersion.DeprecationWarning,
			AdditionalPrinterColumns: crdVersion.AdditionalPrinterColumns,
		}

		if crdVersion.Schema != nil && crdVersion.Schema.OpenAPIV3Schema != nil {
			schema, err := json.Marshal(crdVersion.Schema.OpenAPIV3Schema)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal CRD %s schema for version %q: %w", crd.Name, crdVersion.Name, err)
			}
			apiResourceVersion.Schema.OpenAPIV3Schema.Raw = schema
		}

		if crdVersion.Subresources != nil {
			apiResourceVersion.Subresources = *crdVersion.Subresources
		}

		spec.Versions = append(spec.Versions, apiResourceVersion)

		if onlyFirstServingVersion {
			break
		}
	}

	return spec, nil
}

func APIServiceExportCRDSpecHash(obj *kubebindv1alpha1.APIServiceExportCRDSpec) string {
	bs, err := json.Marshal(obj)
	if err != nil {
		runtime2.HandleError(err)
		return ""
	}

	return toSha224Base62(string(bs))
}

func toSha224Base62(s string) string {
	return toBase62(sha256.Sum224([]byte(s)))
}

func toBase62(hash [28]byte) string {
	var i big.Int
	i.SetBytes(hash[:])
	return i.Text(62)
}
