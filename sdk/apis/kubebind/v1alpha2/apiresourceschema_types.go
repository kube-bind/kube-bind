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
	"encoding/json"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// InformerScope is the scope of the Api.
//
// +kubebuilder:validation:Enum=Cluster;Namespaced
type InformerScope string

const (
	ClusterScope    InformerScope = "Cluster"
	NamespacedScope InformerScope = "Namespaced"
)

// APIResourceSchema
// +crd
// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:scope=Cluster,categories=kube-bindings,shortName=as
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type APIResourceSchema struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec APIResourceSchemaSpec `json:"spec"`
}

type APIResourceSchemaSpec struct {
	// InformerScope indicates whether the informer for defined custom resource is cluster- or namespace-scoped.
	// Allowed values are `Cluster` and `Namespaced`.
	//
	// +required
	// +kubebuilder:validation:Enum=Cluster;Namespaced
	InformerScope InformerScope `json:"informerScope"`

	APIResourceSchemaCRDSpec `json:",inline"`
}

type APIResourceSchemaCRDSpec struct {
	// group is the API group of the defined custom resource. Empty string means the
	// core API group. 	The resources are served under `/apis/<group>/...` or `/api` for the core group.
	//
	// +required
	Group string `json:"group"`

	// names specify the resource and kind names for the custom resource.
	//
	// +required
	Names apiextensionsv1.CustomResourceDefinitionNames `json:"names"`
	// scope indicates whether the defined custom resource is cluster- or namespace-scoped.
	// Allowed values are `Cluster` and `Namespaced`.
	//
	// +required
	// +kubebuilder:validation:Enum=Cluster;Namespaced
	Scope apiextensionsv1.ResourceScope `json:"scope"`

	// versions is the API version of the defined custom resource.
	//
	// Note: the OpenAPI v3 schemas must be equal for all versions until CEL
	//       version migration is supported.
	//
	// +required
	// +listType=map
	// +listMapKey=name
	// +kubebuilder:validation:MinItems=1
	Versions []APIResourceVersion `json:"versions"`

	// conversion defines conversion settings for the defined custom resource.
	// +optional
	Conversion *CustomResourceConversion `json:"conversion,omitempty"`
}

// APIResourceVersion describes one API version of a resource.
type APIResourceVersion struct {
	// name is the version name, e.g. “v1”, “v2beta1”, etc.
	// The custom resources are served under this version at `/apis/<group>/<version>/...` if `served` is true.
	//
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:Pattern=^v[1-9][0-9]*([a-z]+[1-9][0-9]*)?$
	Name string `json:"name"`
	// served is a flag enabling/disabling this version from being served via REST APIs
	//
	// +required
	// +kubebuilder:default=true
	Served bool `json:"served"`
	// storage indicates this version should be used when persisting custom resources to storage.
	// There must be exactly one version with storage=true.
	//
	// +required
	Storage bool `json:"storage"`

	//nolint:gocritic
	// deprecated indicates this version of the custom resource API is deprecated.
	// When set to true, API requests to this version receive a warning header in the server response.
	// Defaults to false.
	//
	// +optional
	Deprecated bool `json:"deprecated,omitempty"`
	// deprecationWarning overrides the default warning returned to API clients.
	// May only be set when `deprecated` is true.
	// The default warning indicates this version is deprecated and recommends use
	// of the newest served version of equal or greater stability, if one exists.
	//
	// +optional
	DeprecationWarning *string `json:"deprecationWarning,omitempty"`
	// schema describes the structural schema used for validation, pruning, and defaulting
	// of this version of the custom resource.
	//
	// +required
	// +kubebuilder:pruning:PreserveUnknownFields
	// +structType=atomic
	Schema CRDVersionSchema `json:"schema"`
	// subresources specify what subresources this version of the defined custom resource have.
	//
	// +optional
	Subresources apiextensionsv1.CustomResourceSubresources `json:"subresources,omitempty"`
	// additionalPrinterColumns specifies additional columns returned in Table output.
	// See https://kubernetes.io/docs/reference/using-api/api-concepts/#receiving-resources-as-tables for details.
	// If no columns are specified, a single column displaying the age of the custom resource is used.
	//
	// +optional
	// +listType=map
	// +listMapKey=name
	AdditionalPrinterColumns []apiextensionsv1.CustomResourceColumnDefinition `json:"additionalPrinterColumns,omitempty"`
}

type CRDVersionSchema struct {
	// openAPIV3Schema is the OpenAPI v3 schema to use for validation and pruning.
	//
	// +kubebuilder:pruning:PreserveUnknownFields
	// +structType=atomic
	// +required
	// +kubebuilder:validation:Required
	OpenAPIV3Schema runtime.RawExtension `json:"openAPIV3Schema"`
}

// CustomResourceConversion describes how to convert different versions of a CR.
// +kubebuilder:validation:XValidation:message="Webhook must be specified if strategy=Webhook",rule="(self.strategy == 'None' && !has(self.webhook))  || (self.strategy == 'Webhook' && has(self.webhook))"
type CustomResourceConversion struct {
	// strategy specifies how custom resources are converted between versions. Allowed values are:
	// - `"None"`: The converter only change the apiVersion and would not touch any other field in the custom resource.
	// - `"Webhook"`: API Server will call to an external webhook to do the conversion. Additional information
	//   is needed for this option. This requires spec.preserveUnknownFields to be false, and spec.conversion.webhook to be set.
	// +kubebuilder:validation:Enum=None;Webhook
	Strategy ConversionStrategyType `json:"strategy"`

	// webhook describes how to call the conversion webhook. Required when `strategy` is set to `"Webhook"`.
	// +optional
	Webhook *WebhookConversion `json:"webhook,omitempty"`
}

// ConversionStrategyType describes different conversion types.
type ConversionStrategyType string

// WebhookConversion describes how to call a conversion webhook
type WebhookConversion struct {
	// clientConfig is the instructions for how to call the webhook if strategy is `Webhook`.
	// +optional
	ClientConfig *WebhookClientConfig `json:"clientConfig,omitempty"`

	// conversionReviewVersions is an ordered list of preferred `ConversionReview`
	// versions the Webhook expects. The API server will use the first version in
	// the list which it supports. If none of the versions specified in this list
	// are supported by API server, conversion will fail for the custom resource.
	// If a persisted Webhook configuration specifies allowed versions and does not
	// include any versions known to the API Server, calls to the webhook will fail.
	// +listType=atomic
	ConversionReviewVersions []string `json:"conversionReviewVersions"`
}

// WebhookClientConfig contains the information to make a TLS connection with the webhook.
type WebhookClientConfig struct {
	// url gives the location of the webhook, in standard URL form
	// (`scheme://host:port/path`).
	//
	// Please note that using `localhost` or `127.0.0.1` as a `host` is
	// risky unless you take great care to run this webhook on all hosts
	// which run an apiserver which might need to make calls to this
	// webhook. Such installs are likely to be non-portable, i.e., not easy
	// to turn up in a new cluster.
	//
	// The scheme must be "https"; the URL must begin with "https://".
	//
	// A path is optional, and if present may be any string permissible in
	// a URL. You may use the path to pass an arbitrary string to the
	// webhook, for example, a cluster identifier.
	//
	// Attempting to use a user or basic auth e.g. "user:password@" is not
	// allowed. Fragments ("#...") and query parameters ("?...") are not
	// allowed, either.
	//
	// +kubebuilder:validation:Format=uri
	URL *string `json:"url,omitempty"`

	// caBundle is a PEM encoded CA bundle which will be used to validate the webhook's server certificate.
	// If unspecified, system trust roots on the apiserver are used.
	// +optional
	CABundle []byte `json:"caBundle,omitempty"`
}

// APIResourceSchemaList is a list of APIResourceSchemas.
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type APIResourceSchemaList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []APIResourceSchema `json:"items"`
}

func (v *APIResourceVersion) GetSchema() (*apiextensionsv1.JSONSchemaProps, error) {
	if v.Schema.OpenAPIV3Schema.Raw == nil {
		return nil, nil
	}
	var schema apiextensionsv1.JSONSchemaProps
	if err := json.Unmarshal(v.Schema.OpenAPIV3Schema.Raw, &schema); err != nil {
		return nil, err
	}
	return &schema, nil
}

func (v *APIResourceVersion) SetSchema(schema *apiextensionsv1.JSONSchemaProps) error {
	if schema == nil {
		v.Schema.OpenAPIV3Schema.Raw = nil
		return nil
	}
	raw, err := json.Marshal(schema)
	if err != nil {
		return err
	}
	v.Schema.OpenAPIV3Schema.Raw = raw
	return nil
}
