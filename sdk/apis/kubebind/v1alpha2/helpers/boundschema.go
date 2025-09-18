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
	"crypto/sha256"
	"encoding/json"
	"fmt"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	kubebindv1alpha2 "github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2"
)

// CRDToBoundSchema converts a CustomResourceDefinition to an BoundSchema.
func CRDToBoundSchema(crd *apiextensionsv1.CustomResourceDefinition, prefix string) (*kubebindv1alpha2.BoundSchema, error) {
	name := crd.Name
	if prefix != "" {
		name = prefix + "." + crd.Name
	}
	// Derive informer scope from the CRD scope.
	informerScope := kubebindv1alpha2.InformerScope(crd.Spec.Scope)
	boundSchema := &kubebindv1alpha2.BoundSchema{
		TypeMeta: metav1.TypeMeta{
			APIVersion: kubebindv1alpha2.SchemeGroupVersion.String(),
			Kind:       "BoundSchema",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: kubebindv1alpha2.BoundSchemaSpec{
			InformerScope: informerScope,
			APICRDSpec: kubebindv1alpha2.APICRDSpec{
				Group: crd.Spec.Group,
				Names: crd.Spec.Names,
				Scope: crd.Spec.Scope,
			},
		},
	}

	if len(crd.Spec.Versions) == 0 {
		return nil, fmt.Errorf("no versions specified for CRD %q", crd.Name)
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

		boundSchema.Spec.Conversion = crConversion
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
			apiResourceVersion.Schema = runtime.RawExtension{Raw: rawSchema}
		}

		if crdVersion.Subresources != nil {
			apiResourceVersion.Subresources = *crdVersion.Subresources
		}

		boundSchema.Spec.Versions = append(boundSchema.Spec.Versions, apiResourceVersion)
	}

	return boundSchema, nil
}

func UnstructuredToBoundSchema(u unstructured.Unstructured) (*kubebindv1alpha2.BoundSchema, error) {
	boundSchema := &kubebindv1alpha2.BoundSchema{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.UnstructuredContent(), boundSchema); err != nil {
		return nil, err
	}
	return boundSchema, nil
}

func BoundSchemasSpecHash(schemas []*kubebindv1alpha2.BoundSchema) (string, error) {
	hash := sha256.New()
	for _, schema := range schemas {
		if err := json.NewEncoder(hash).Encode(schema); err != nil {
			return "", fmt.Errorf("failed to encode schema %s: %w", schema.Name, err)
		}
	}
	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

func BoundSchemaToCRD(schema *kubebindv1alpha2.BoundSchema) *apiextensionsv1.CustomResourceDefinition {
	crd := &apiextensionsv1.CustomResourceDefinition{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apiextensions.k8s.io/v1",
			Kind:       "CustomResourceDefinition",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: schema.Spec.Names.Plural + "." + schema.Spec.Group,
		},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Group: schema.Spec.Group,
			Names: schema.Spec.Names,
			Scope: schema.Spec.Scope,
		},
	}

	if schema.Spec.Conversion != nil {
		crConversion := &apiextensionsv1.CustomResourceConversion{
			Strategy: apiextensionsv1.ConversionStrategyType(schema.Spec.Conversion.Strategy),
		}

		if apiextensionsv1.ConversionStrategyType(schema.Spec.Conversion.Strategy) == apiextensionsv1.WebhookConverter && schema.Spec.Conversion.Webhook != nil {
			crConversion.Webhook = &apiextensionsv1.WebhookConversion{
				ConversionReviewVersions: schema.Spec.Conversion.Webhook.ConversionReviewVersions,
			}

			if schema.Spec.Conversion.Webhook.ClientConfig != nil {
				crConversion.Webhook.ClientConfig = &apiextensionsv1.WebhookClientConfig{
					URL:      schema.Spec.Conversion.Webhook.ClientConfig.URL,
					CABundle: schema.Spec.Conversion.Webhook.ClientConfig.CABundle,
				}
			}
		}

		crd.Spec.Conversion = crConversion
	}

	for _, version := range schema.Spec.Versions {
		crdVersion := apiextensionsv1.CustomResourceDefinitionVersion{
			Name:                     version.Name,
			Served:                   version.Served,
			Storage:                  version.Storage,
			Deprecated:               version.Deprecated,
			DeprecationWarning:       version.DeprecationWarning,
			AdditionalPrinterColumns: version.AdditionalPrinterColumns,
			Subresources:             &version.Subresources,
		}
		// Now schema can be openapiv3 or v2.
		// we do some poor man checking:
		if len(version.Schema.Raw) > 0 {
			// Try to unmarshal as CustomResourceValidation first (contains openAPIV3Schema field)
			var validation apiextensionsv1.CustomResourceValidation
			if err := json.Unmarshal(version.Schema.Raw, &validation); err == nil && validation.OpenAPIV3Schema != nil {
				crdVersion.Schema = &validation
			} else {
				// Fall back to direct JSONSchemaProps
				var jsonSchema apiextensionsv1.JSONSchemaProps
				if err := json.Unmarshal(version.Schema.Raw, &jsonSchema); err == nil {
					crdVersion.Schema = &apiextensionsv1.CustomResourceValidation{
						OpenAPIV3Schema: &jsonSchema,
					}
				}
			}
		}

		crd.Spec.Versions = append(crd.Spec.Versions, crdVersion)
	}

	return crd
}
