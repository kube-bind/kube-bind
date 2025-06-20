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
	"math/big"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	kubebindv1alpha2 "github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2"
)

// CRDToAPIResourceSchema converts a CustomResourceDefinition to an APIResourceSchema.
func CRDToAPIResourceSchema(crd *apiextensionsv1.CustomResourceDefinition, prefix string) (*kubebindv1alpha2.APIResourceSchema, error) {
	name := crd.Name
	if prefix != "" {
		name = prefix + "." + crd.Name
	}

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

// APIResourceSchemaToCRD converts an APIResourceSchema to a CustomResourceDefinition.
func APIResourceSchemaToCRD(schema *kubebindv1alpha2.APIResourceSchema) (*apiextensionsv1.CustomResourceDefinition, error) {
	crd := &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: schema.Spec.Names.Plural + "." + schema.Spec.Group,
			Annotations: map[string]string{
				"kube-bind.io/source-schema": schema.Name,
			},
		},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Group: schema.Spec.Group,
			Names: schema.Spec.Names,
			Scope: schema.Spec.Scope,
		},
	}

	for _, schemaVersion := range schema.Spec.Versions {
		crdVersion := apiextensionsv1.CustomResourceDefinitionVersion{
			Name:                     schemaVersion.Name,
			Served:                   schemaVersion.Served,
			Storage:                  schemaVersion.Storage,
			Deprecated:               schemaVersion.Deprecated,
			DeprecationWarning:       schemaVersion.DeprecationWarning,
			AdditionalPrinterColumns: schemaVersion.AdditionalPrinterColumns,
		}

		if schemaVersion.Schema.OpenAPIV3Schema.Raw != nil {
			var schemaObj apiextensionsv1.JSONSchemaProps
			if err := json.Unmarshal(schemaVersion.Schema.OpenAPIV3Schema.Raw, &schemaObj); err != nil {
				return nil, fmt.Errorf("failed to unmarshal schema for version %s: %v", schemaVersion.Name, err)
			}
			crdVersion.Schema = &apiextensionsv1.CustomResourceValidation{
				OpenAPIV3Schema: &schemaObj,
			}
		}

		if schemaVersion.Subresources.Status != nil || schemaVersion.Subresources.Scale != nil {
			crdVersion.Subresources = &apiextensionsv1.CustomResourceSubresources{}

			if schemaVersion.Subresources.Status != nil {
				crdVersion.Subresources.Status = &apiextensionsv1.CustomResourceSubresourceStatus{}
			}

			if schemaVersion.Subresources.Scale != nil {
				crdVersion.Subresources.Scale = &apiextensionsv1.CustomResourceSubresourceScale{
					SpecReplicasPath:   schemaVersion.Subresources.Scale.SpecReplicasPath,
					StatusReplicasPath: schemaVersion.Subresources.Scale.StatusReplicasPath,
					LabelSelectorPath:  schemaVersion.Subresources.Scale.LabelSelectorPath,
				}
			}
		}

		crd.Spec.Versions = append(crd.Spec.Versions, crdVersion)
	}

	return crd, nil
}
func APIResourceSchemaCRDSpecHash(obj *kubebindv1alpha2.APIResourceSchemaCRDSpec) string {
	bs, err := json.Marshal(obj)
	if err != nil {
		utilruntime.HandleError(err)
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
