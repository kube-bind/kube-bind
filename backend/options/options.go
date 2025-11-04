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

package options

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/component-base/logs"
	logsv1 "k8s.io/component-base/logs/api/v1"

	kubebindv1alpha2 "github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2"
)

type Options struct {
	Logs   *logs.Options
	OIDC   *OIDC
	Cookie *Cookie
	Serve  *Serve

	ExtraOptions
}

type ExtraOptions struct {
	KubeConfig string

	Provider  string
	ServerURL string

	NamespacePrefix        string
	PrettyName             string
	ConsumerScope          string
	ClusterScopedIsolation string
	ExternalAddress        string
	ExternalCAFile         string
	ExternalCA             []byte
	TLSExternalServerName  string
	// Defines the source of the schema for the bind screen.
	// Options are:
	// CustomResourceDefinition.v1.apiextensions.k8s.io
	// APIResourceSchema.v1alpha1.apis.kcp.io
	SchemaSource string

	TestingAutoSelect         string
	TestingSkipNameValidation bool

	// If ControllerFrontend starts with http:// it is treated as a URL to a SPA server
	// Else - it is treated as a path to static files to be served.
	Frontend string
}

type completedOptions struct {
	Logs   *logs.Options
	OIDC   *OIDC
	Cookie *Cookie
	Serve  *Serve

	ExtraOptions
}

type CompletedOptions struct {
	*completedOptions
}

func NewOptions() *Options {
	// Default to -v=2
	logs := logs.NewOptions()
	logs.Verbosity = logsv1.VerbosityLevel(2)

	return &Options{
		Logs:   logs,
		OIDC:   NewOIDC(),
		Cookie: NewCookie(),
		Serve:  NewServe(),

		ExtraOptions: ExtraOptions{
			Provider:               "kubernetes",
			NamespacePrefix:        "cluster",
			PrettyName:             "Backend",
			ConsumerScope:          string(kubebindv1alpha2.NamespacedScope),
			ClusterScopedIsolation: string(kubebindv1alpha2.IsolationPrefixed),
			ServerURL:              "",
			SchemaSource:           CustomResourceDefinitionSource.String(),
			Frontend:               "/www",
		},
	}
}

var providerAliases = map[string]string{
	"kcp":        "kcp",
	"kubernetes": "kubernetes",
	"":           "kubernetes",
}

type SchemaSource string

func (s SchemaSource) String() string {
	return string(s)
}

var (
	KCPAPIResourceSchemaSource     = SchemaSource("APIResourceSchema.v1alpha1.apis.kcp.io")
	CustomResourceDefinitionSource = SchemaSource("CustomResourceDefinition.v1.apiextensions.k8s.io")
)

// TODO(mjudeikis): https://github.com/kube-bind/kube-bind/issues/298
// We should relax these once we happy they work with any schema.
var schemaSourceAliases = map[string]string{
	CustomResourceDefinitionSource.String(): CustomResourceDefinitionSource.String(), // mostrly for e2e tests
	"customresourcedefinitions":             CustomResourceDefinitionSource.String(),
	"apiresourceschemas":                    KCPAPIResourceSchemaSource.String(),
	KCPAPIResourceSchemaSource.String():     KCPAPIResourceSchemaSource.String(),
}

func (options *Options) AddFlags(fs *pflag.FlagSet) {
	logsv1.AddFlags(options.Logs, fs)
	options.OIDC.AddFlags(fs)
	options.Cookie.AddFlags(fs)
	options.Serve.AddFlags(fs)

	fs.StringVar(&options.KubeConfig, "kubeconfig", options.KubeConfig, "path to a kubeconfig. Only required if out-of-cluster")
	fs.StringVar(&options.NamespacePrefix, "namespace-prefix", options.NamespacePrefix, "The prefix to use for cluster namespaces")
	fs.StringVar(&options.PrettyName, "pretty-name", options.PrettyName, "Pretty name for the backend")
	fs.StringVar(&options.ConsumerScope, "consumer-scope", options.ConsumerScope, "How consumers access the service provider cluster. In Kubernetes, \"namespaced\" allows namespace isolation. In kcp, \"cluster\" allows workspace isolation, and with that allows cluster-scoped resources to bind and it is generally more performant.")
	fs.StringVar(&options.ClusterScopedIsolation, "cluster-scoped-isolation", options.ClusterScopedIsolation, "How cluster scoped service objects are isolated between multiple consumers on the provider side. Among the choices, \"prefixed\" prepends the name of the cluster namespace to an object's name; \"namespaced\" maps a consumer side object into a namespaced object inside the corresponding cluster namespace; \"none\" is used for the case of a dedicated provider where isolation is not necessary.")
	fs.StringVar(&options.ExternalAddress, "external-address", options.ExternalAddress, "The external address for the service provider cluster, including https:// and port. If not specified, service account's hosts are used.")
	fs.StringVar(&options.ExternalCAFile, "external-ca-file", options.ExternalCAFile, "The external CA file for the service provider cluster. If not specified, service account's CA is used.")
	fs.StringVar(&options.TLSExternalServerName, "external-server-name", options.TLSExternalServerName, "The external (TLS) server name used by consumers to talk to the service provider cluster. This can be useful to select the right certificate via SNI.")
	fs.StringVar(&options.Frontend, "frontend", options.Frontend, "If starts with http:// it is treated as a URL to a SPA server Else - it is treated as a path to static files to be served.")

	fs.StringVar(&options.Provider, "multicluster-runtime-provider", options.Provider,
		fmt.Sprintf("The multicluster runtime provider. Possible values are: %v", sets.List(sets.Set[string](sets.StringKeySet(providerAliases)))),
	)

	values := make([]string, 0, len(schemaSourceAliases))
	for _, v := range schemaSourceAliases {
		values = append(values, v)
	}

	fs.StringVar(&options.SchemaSource, "schema-source", options.SchemaSource,
		fmt.Sprintf("Defines the source of the schema in Kind.Version.Group format for the bind screen. Defaults to CustomResourceDefinition.v1.apiextensions.k8s.io. Possible values are: %v",
			values),
	)

	fs.StringVar(&options.ServerURL, "server-url", options.ServerURL, "The URL of the backend server. If not specified, it will be derived from the kubeconfig or service account's hosts.")

	fs.StringVar(&options.TestingAutoSelect, "testing-auto-select", options.TestingAutoSelect, "<resource>.<group> that is automatically selected on th bind screen for testing")
	fs.MarkHidden("testing-auto-select") //nolint:errcheck
}

func (options *Options) Complete() (*CompletedOptions, error) {
	if err := options.OIDC.Complete(); err != nil {
		return nil, err
	}
	if err := options.Cookie.Complete(); err != nil {
		return nil, err
	}
	if err := options.Serve.Complete(); err != nil {
		return nil, err
	}

	// normalize the scope and the isolation
	if strings.ToLower(options.ConsumerScope) == "namespaced" {
		options.ConsumerScope = string(kubebindv1alpha2.NamespacedScope)
	}
	if strings.ToLower(options.ConsumerScope) == "cluster" {
		options.ConsumerScope = string(kubebindv1alpha2.ClusterScope)
	}
	switch strings.ToLower(options.ClusterScopedIsolation) {
	case "prefixed":
		options.ClusterScopedIsolation = string(kubebindv1alpha2.IsolationPrefixed)
	case "namespaced":
		options.ClusterScopedIsolation = string(kubebindv1alpha2.IsolationNamespaced)
	case "none":
		options.ClusterScopedIsolation = string(kubebindv1alpha2.IsolationNone)
	}

	if options.ExternalCAFile != "" && options.ExternalCA != nil {
		return nil, fmt.Errorf("cannot specify both --external-ca-file and set ExternalCA")
	}
	if options.ExternalCAFile != "" {
		ca, err := os.ReadFile(options.ExternalCAFile)
		if err != nil {
			return nil, fmt.Errorf("error reading external CA file: %v", err)
		}
		options.ExternalCA = ca
	}
	return &CompletedOptions{
		completedOptions: &completedOptions{
			Logs:         options.Logs,
			OIDC:         options.OIDC,
			Cookie:       options.Cookie,
			Serve:        options.Serve,
			ExtraOptions: options.ExtraOptions,
		},
	}, nil
}

func (options *CompletedOptions) Validate() error {
	if options.NamespacePrefix == "" {
		return fmt.Errorf("namespace prefix cannot be empty")
	}
	if options.PrettyName == "" {
		return fmt.Errorf("pretty name cannot be empty")
	}

	if err := options.OIDC.Validate(); err != nil {
		return err
	}
	if err := options.Cookie.Validate(); err != nil {
		return err
	}
	if options.ConsumerScope != string(kubebindv1alpha2.NamespacedScope) && options.ConsumerScope != string(kubebindv1alpha2.ClusterScope) {
		return fmt.Errorf("consumer scope must be either %q or %q", kubebindv1alpha2.NamespacedScope, kubebindv1alpha2.ClusterScope)
	}

	if options.ExternalAddress != "" {
		if !strings.HasPrefix(options.ExternalAddress, "https://") {
			return fmt.Errorf("external hostname must start with https://")
		}
		_, err := url.Parse(options.ExternalAddress)
		if err != nil {
			return fmt.Errorf("invalid external hostname: %v", err)
		}
	}

	provider := providerAliases[options.Provider]
	if provider == "" {
		return fmt.Errorf("unknown provider %q, must be one of %v", options.Provider, sets.List(sets.Set[string](sets.StringKeySet(providerAliases))))
	}
	options.Provider = provider

	schemaSource := schemaSourceAliases[options.SchemaSource]
	if schemaSource == "" {
		return fmt.Errorf("unknown schema source %q, must be one of %v", options.SchemaSource, sets.List(sets.Set[string](sets.StringKeySet(schemaSourceAliases))))
	}
	options.SchemaSource = schemaSource
	parts := strings.SplitN(options.SchemaSource, ".", 3)
	if len(parts) != 3 { // We check this in validation, but just in case.
		return fmt.Errorf("invalid schema source: %q", options.SchemaSource)
	}

	return nil
}
