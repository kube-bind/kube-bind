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

package helpers

import (
	"encoding/json"
	"fmt"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	kubebindv1alpha2 "github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2"
)

// CRDToBoundAPIResourceSchema converts a CustomResourceDefinition to an BoundAPIResourceSchema.
func CRDToBoundAPIResourceSchema(crd *apiextensionsv1.CustomResourceDefinition, prefix string) (*kubebindv1alpha2.APIResourceSchema, error) {
	name := prefix + "." + crd.Name
	informerScope := kubebindv1alpha2.NamespacedScope
	apiResourceSchema := &kubebindv1alpha2.APIResourceSchema{
		TypeMeta: metav1.TypeMeta{
			APIVersion: kubebindv1alpha2.SchemeGroupVersion.String(),
			Kind:       "APIResourceSchema",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: kubebindv1alpha2.APIResourceSchemaSpec{
			InformerScope: informerScope,
			APIResourceSchemaCRDSpec: kubebindv1alpha2.APIResourceSchemaCRDSpec{
				Group: crd.Spec.Group,
				Names: crd.Spec.Names,
				Scope: crd.Spec.Scope,
			},
		},
	}

	if len(crd.Spec.Versions) > 1 && crd.Spec.Conversion == nil {
		return nil, fmt.Errorf("multiple versions specified for CRD %q but no conversion strategy", crd.Name)
	}

	if crd.Spec.Conversion != nil {
		crConversion := &kubebindv1alpha2.CustomResourceConversion{
			Strategy: kubebindv1alpha2.ConversionStrategyType(crd.Spec.Conversion.Strategy),
		}

		if crd.Spec.Conversion.Strategy == "Webhook" {
			crConversion.Webhook = &kubebindv1alpha2.WebhookConversion{
				ConversionReviewVersions: crd.Spec.Conversion.Webhook.ConversionReviewVersions,
			}

			if crd.Spec.Conversion.Webhook.ClientConfig != nil {
				crConversion.Webhook.ClientConfig = &kubebindv1alpha2.WebhookClientConfig{
					URL:      crd.Spec.Conversion.Webhook.ClientConfig.URL,
					CABundle: crd.Spec.Conversion.Webhook.ClientConfig.CABundle,
				}
			}
		}

		apiResourceSchema.Spec.Conversion = crConversion
	}

	for _, crdVersion := range crd.Spec.Versions {
		apiResourceVersion := kubebindv1alpha2.APIResourceVersion{
			Name:                     crdVersion.Name,
			Served:                   crdVersion.Served,
			Storage:                  crdVersion.Storage,
			Deprecated:               crdVersion.Deprecated,
			DeprecationWarning:       crdVersion.DeprecationWarning,
			AdditionalPrinterColumns: crdVersion.AdditionalPrinterColumns,
		}
		if crdVersion.Schema != nil && crdVersion.Schema.OpenAPIV3Schema != nil {
			rawSchema, err := json.Marshal(crdVersion.Schema.OpenAPIV3Schema)
			if err != nil {
				return nil, fmt.Errorf("error converting schema for version %q: %w", crdVersion.Name, err)
			}
			apiResourceVersion.Schema = kubebindv1alpha2.CRDVersionSchema{
				OpenAPIV3Schema: runtime.RawExtension{Raw: rawSchema},
			}
		}

		if crdVersion.Subresources != nil {
			apiResourceVersion.Subresources = *crdVersion.Subresources
		}

		apiResourceSchema.Spec.Versions = append(apiResourceSchema.Spec.Versions, apiResourceVersion)
	}

	return apiResourceSchema, nil
}
