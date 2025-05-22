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
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"
	"k8s.io/component-base/logs"
	logsv1 "k8s.io/component-base/logs/api/v1"
	"k8s.io/component-base/version"

	"github.com/kube-bind/kube-bind/cli/pkg/kubectl/base"
	kubebindv1alpha2 "github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2"
)

type CRD2APIResourceSchemaOptions struct {
	Options *base.Options
	Logs    *logs.Options
	Print   *genericclioptions.PrintFlags

	// GenerateInCluster indicates whether to generate the APIResourceSchema in-cluster.
	GenerateInCluster bool
	// OutputDir is the directory where the APIResourceSchemas will be written.
	OutputDir string
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

	cmd.Flags().BoolVar(&b.GenerateInCluster, "generate-in-cluster", b.GenerateInCluster, "Generate the APIResourceSchema in-cluster.")
	cmd.Flags().StringVar(&b.OutputDir, "output-dir", b.OutputDir, "Directory where APIResourceSchemas will be written.")
}

func (b *CRD2APIResourceSchemaOptions) Complete(args []string) error {
	return b.Options.Complete()
}

func (b *CRD2APIResourceSchemaOptions) Validate() error {
	if b.GenerateInCluster && b.OutputDir != "" {
		return errors.New("output-dir and generate-in-cluster cannot be used together")
	}

	return b.Options.Validate()
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

	if b.OutputDir == "" {
		b.OutputDir = "."
	}

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

		prefix := fmt.Sprintf("v%s-%s", time.Now().Format("060102"), string(version.Get().GitCommit))
		apiResourceSchema, err := convertCRDToAPIResourceSchema(crdObj, prefix)
		if err != nil {
			fmt.Fprintf(b.Options.ErrOut, "Failed to convert CRD %s to APIResourceSchema: %v\n", crdObj.Name, err)
			continue
		}

		if apiResourceSchema == nil {
			fmt.Fprintf(b.Options.ErrOut, "Skipping CRD %s: no schema found\n", crdObj.Name)
			continue
		}

		if b.GenerateInCluster {
			if err := generateAPIResourceSchemaInCluster(ctx, client, apiResourceSchema, b.Options.ErrOut, b.Options.Out); err != nil {
				continue
			}
		}
		if err := writeObjectToYAML(b.OutputDir, apiResourceSchema, b.Options.Out); err != nil {
			return err
		}
	}

	return nil
}

func generateAPIResourceSchemaInCluster(ctx context.Context, client dynamic.Interface, apiResourceSchema *kubebindv1alpha2.APIResourceSchema, errOut, out io.Writer) error {
	apiResourceSchemaGVR := kubebindv1alpha2.SchemeGroupVersion.WithResource("apiresourceschemas")
	unstructuredObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(apiResourceSchema)
	if err != nil {
		return fmt.Errorf("failed to convert APIResourceSchema to unstructured: %w", err)
	}
	unstructuredResource := &unstructured.Unstructured{Object: unstructuredObj}

	_, err = client.Resource(apiResourceSchemaGVR).Create(ctx, unstructuredResource, metav1.CreateOptions{})
	if err != nil {
		fmt.Fprintf(errOut, "Failed to create APIResourceSchema for CRD %s: %v\n", apiResourceSchema.Name, err)
		return err
	}

	fmt.Fprintf(out, "Successfully created APIResourceSchema for CRD %s\n", apiResourceSchema.Name)
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

func writeObjectToYAML(outputDir string, apiResourceSchema *kubebindv1alpha2.APIResourceSchema, logger io.Writer) error {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory %s: %w", outputDir, err)
	}

	scheme := runtime.NewScheme()
	if err := kubebindv1alpha2.AddToScheme(scheme); err != nil {
		return fmt.Errorf("failed to register kubebindv1alpha2 API group: %w", err)
	}

	codecs := serializer.NewCodecFactory(scheme)
	info, ok := runtime.SerializerInfoForMediaType(codecs.SupportedMediaTypes(), runtime.ContentTypeYAML)
	if !ok {
		return fmt.Errorf("unsupported media type %q", runtime.ContentTypeYAML)
	}
	encoder := codecs.EncoderForVersion(info.Serializer, kubebindv1alpha2.SchemeGroupVersion)

	out, err := runtime.Encode(encoder, apiResourceSchema)
	if err != nil {
		return fmt.Errorf("failed to encode APIResourceSchema %s: %w", apiResourceSchema.Name, err)
	}
	outputPath := filepath.Join(outputDir, fmt.Sprintf("%s.yaml", apiResourceSchema.Name))
	if err := os.WriteFile(outputPath, out, 0644); err != nil {
		return fmt.Errorf("failed to write APIResourceSchema to file %s: %w", outputPath, err)
	}

	fmt.Fprintf(logger, "Wrote APIResourceSchema %s to %s\n", apiResourceSchema.Name, outputPath)
	return nil
}
