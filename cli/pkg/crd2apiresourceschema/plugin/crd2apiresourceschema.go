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

package plugin

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"
	"k8s.io/component-base/logs"
	logsv1 "k8s.io/component-base/logs/api/v1"

	"github.com/kube-bind/kube-bind/cli/pkg/kubectl/base"
	kubebindv1alpha2 "github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2"
)

type CRD2APIResourceSchemaOptions struct {
	Options *base.Options
	Logs    *logs.Options
	Print   *genericclioptions.PrintFlags
}

func NewCRD2APIResourceSchemaOptions(streams genericclioptions.IOStreams) *CRD2APIResourceSchemaOptions {
	return &CRD2APIResourceSchemaOptions{
		Options: base.NewOptions(streams),
		Logs:    logs.NewOptions(),
		Print:   genericclioptions.NewPrintFlags("crd2apiresourceschema"),
	}
}

func (b *CRD2APIResourceSchemaOptions) AddCmdFlags(cmd *cobra.Command) {
	b.Options.BindFlags(cmd)
	logsv1.AddFlags(b.Logs, cmd.Flags())
	b.Print.AddFlags(cmd)
}

func (b *CRD2APIResourceSchemaOptions) Complete(args []string) error {
	if err := b.Options.Complete(); err != nil {
		return err
	}

	return nil
}

// Run starts the process of converting CRDs to APIResourceSchema objects.
func (b *CRD2APIResourceSchemaOptions) Run(ctx context.Context) error {
	config, err := b.Options.ClientConfig.ClientConfig()
	if err != nil {
		return err
	}
	client, err := dynamic.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create dynamic client: %w", err)
	}

	apiResourceSchemaGVR := kubebindv1alpha2.SchemeGroupVersion.WithResource("apiresourceschemas")
	crdGVR := apiextensionsv1.SchemeGroupVersion.WithResource("customresourcedefinitions")
	crdList, err := client.Resource(crdGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list CRDs: %w", err)
	}

	for _, crd := range crdList.Items {
		crdObj := &apiextensionsv1.CustomResourceDefinition{}
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(crd.UnstructuredContent(), crdObj); err != nil {
			return fmt.Errorf("failed to convert CRD: %w", err)
		}

		if crdObj.Spec.Group == "kube-bind.io" {
			fmt.Fprintf(b.Options.ErrOut, "Skipping CRD %s: belongs to group kube-bind.io\n", crdObj.Name)
			continue
		}

		apiResourceSchema, err := convertCRDToAPIResourceSchema(crdObj, "apiresourceschema")
		if err != nil {
			fmt.Fprintf(b.Options.ErrOut, "Failed to convert CRD %s to APIResourceSchema: %v\n", crdObj.Name, err)
		}

		if apiResourceSchema == nil {
			fmt.Fprintf(b.Options.ErrOut, "Skipping CRD %s: no schema found\n", crdObj.Name)
			continue
		}

		unstructuredObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(apiResourceSchema)
		if err != nil {
			return fmt.Errorf("failed to convert APIResourceSchema to unstructured: %w", err)
		}
		unstructuredResource := &unstructured.Unstructured{Object: unstructuredObj}

		_, err = client.Resource(apiResourceSchemaGVR).Create(ctx, unstructuredResource, metav1.CreateOptions{})
		if err != nil {
			fmt.Fprintf(b.Options.ErrOut, "Failed to create APIResourceSchema for CRD %s: %v\n", crdObj.Name, err)
			continue
		}

		fmt.Fprintf(b.Options.Out, "Successfully created APIResourceSchema for CRD %s\n", crdObj.Name)
	}

	return nil
}

func convertCRDToAPIResourceSchema(crd *apiextensionsv1.CustomResourceDefinition, prefix string) (*kubebindv1alpha2.APIResourceSchema, error) {
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
		return nil, fmt.Errorf("multiple versions specified but no conversion strategy")
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

	for i := range crd.Spec.Versions {
		crdVersion := crd.Spec.Versions[i]

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

		apiResourceSchema.Spec.Versions = append(apiResourceSchema.Spec.Versions, apiResourceVersion)
	}

	return apiResourceSchema, nil
}
