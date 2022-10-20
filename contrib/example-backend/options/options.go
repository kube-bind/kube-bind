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

package options

import (
	"fmt"

	"github.com/spf13/pflag"

	"k8s.io/component-base/logs"
	logsv1 "k8s.io/component-base/logs/api/v1"

	kubebindv1alpha1 "github.com/kube-bind/kube-bind/pkg/apis/kubebind/v1alpha1"
)

type Options struct {
	Logs  *logs.Options
	OIDC  *OIDC
	Serve *Serve

	ExtraOptions
}
type ExtraOptions struct {
	KubeConfig string

	NamespacePrefix string
	PrettyName      string
	ConsumerScope   string

	TestingAutoSelect string
}

type completedOptions struct {
	Logs  *logs.Options
	OIDC  *OIDC
	Serve *Serve

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
		Logs:  logs,
		OIDC:  NewOIDC(),
		Serve: NewServe(),

		ExtraOptions: ExtraOptions{
			NamespacePrefix: "cluster",
			PrettyName:      "Example Backend",
			ConsumerScope:   string(kubebindv1alpha1.NamespacedScope),
		},
	}
}

func (options *Options) AddFlags(fs *pflag.FlagSet) {
	logsv1.AddFlags(options.Logs, fs)
	options.OIDC.AddFlags(fs)
	options.Serve.AddFlags(fs)

	fs.StringVar(&options.KubeConfig, "kubeconfig", options.KubeConfig, "path to a kubeconfig. Only required if out-of-cluster")
	fs.StringVar(&options.NamespacePrefix, "namespace-prefix", options.NamespacePrefix, "The prefix to use for cluster namespaces")
	fs.StringVar(&options.PrettyName, "pretty-name", options.PrettyName, "Pretty name for the backend")
	fs.StringVar(&options.ConsumerScope, "consumer-scope", options.ConsumerScope, "How consumers access the service provider cluster. In Kubernetes Namespaced allows namespace isolation. In kcp Cluster allows workspace isolation, and with that allows cluster-scoped resources to bind and it is generally more performant.")

	fs.StringVar(&options.TestingAutoSelect, "testing-auto-select", options.TestingAutoSelect, "<resource>.<group> that is automatically selected on th bind screen for testing")
	fs.MarkHidden("testing-auto-select") // nolint: errcheck
}

func (options *Options) Complete() (*CompletedOptions, error) {
	if err := options.OIDC.Complete(); err != nil {
		return nil, err
	}
	if err := options.Serve.Complete(); err != nil {
		return nil, err
	}

	return &CompletedOptions{
		completedOptions: &completedOptions{
			Logs:         options.Logs,
			OIDC:         options.OIDC,
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
	if options.ConsumerScope != string(kubebindv1alpha1.NamespacedScope) && options.ConsumerScope != string(kubebindv1alpha1.ClusterScope) {
		return fmt.Errorf("consumer scope must be either %q or %q", kubebindv1alpha1.NamespacedScope, kubebindv1alpha1.ClusterScope)
	}

	return nil
}
