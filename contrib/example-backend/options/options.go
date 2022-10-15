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
)

type Options struct {
	Logs *logs.Options
	OIDC *OIDC

	ExtraOptions
}
type ExtraOptions struct {
	ListenIP   string
	ListenPort int

	KubeConfig string

	NamespacePrefix string
	PrettyName      string
}

type completedOptions struct {
	Logs *logs.Options
	OIDC *OIDC

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
		Logs: logs,
		OIDC: NewOIDC(),

		ExtraOptions: ExtraOptions{
			ListenIP:   "127.0.0.1",
			ListenPort: 8080,

			NamespacePrefix: "cluster",
			PrettyName:      "Example Backend",
		},
	}
}

func (options *Options) AddFlags(fs *pflag.FlagSet) {
	logsv1.AddFlags(options.Logs, fs)
	options.OIDC.AddFlags(fs)

	fs.StringVar(&options.ListenIP, "listen-ip", options.ListenIP, "The host IP where the backend is running")
	fs.IntVar(&options.ListenPort, "listen-port", options.ListenPort, "The host port where the backend is running")
	fs.StringVar(&options.KubeConfig, "kubeconfig", options.KubeConfig, "path to a kubeconfig. Only required if out-of-cluster")
	fs.StringVar(&options.NamespacePrefix, "namespace-prefix", options.NamespacePrefix, "The prefix to use for cluster namespaces")
	fs.StringVar(&options.PrettyName, "pretty-name", options.PrettyName, "Pretty name for the backend")
}

func (options *Options) Complete() (*CompletedOptions, error) {
	if err := options.OIDC.Complete(); err != nil {
		return nil, err
	}

	if options.OIDC.CallbackURL == "" {
		options.OIDC.CallbackURL = fmt.Sprintf("http://%s:%d/callback", options.ListenIP, options.ListenPort)
	}

	return &CompletedOptions{
		completedOptions: &completedOptions{
			Logs:         options.Logs,
			OIDC:         options.OIDC,
			ExtraOptions: options.ExtraOptions,
		},
	}, nil
}

func (options *CompletedOptions) Validate() error {
	if options.ListenIP == "" {
		return fmt.Errorf("listen IP cannot be empty")
	}
	if options.ListenPort == 0 {
		return fmt.Errorf("listen port cannot be empty")
	}
	if options.NamespacePrefix == "" {
		return fmt.Errorf("namespace prefix cannot be empty")
	}
	if options.PrettyName == "" {
		return fmt.Errorf("pretty name cannot be empty")
	}

	if err := options.OIDC.Validate(); err != nil {
		return err
	}

	return nil
}
