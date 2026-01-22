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
	"math/rand"
	"os"
	"time"

	"github.com/spf13/pflag"
	"k8s.io/component-base/logs"
	logsv1 "k8s.io/component-base/logs/api/v1"
)

type Options struct {
	Logs *logs.Options

	ExtraOptions
}

type ExtraOptions struct {
	KubeConfigPath string

	LeaseLockName      string
	LeaseLockNamespace string
	LeaseLockIdentity  string

	ServerAddr              string
	ProviderPollingInterval time.Duration
}

type completedOptions struct {
	Logs *logs.Options

	ExtraOptions
}

type CompletedOptions struct {
	*completedOptions
}

func NewOptions() *Options {
	// Default to -v=2
	logs := logs.NewOptions()
	logs.Verbosity = logsv1.VerbosityLevel(2)

	opts := &Options{
		Logs: logs,

		ExtraOptions: ExtraOptions{
			LeaseLockName:      "kube-bind",
			LeaseLockNamespace: os.Getenv("POD_NAMESPACE"),
			LeaseLockIdentity:  os.Getenv("POD_NAME"),
		},
	}

	if opts.LeaseLockNamespace == "" {
		opts.LeaseLockNamespace = "kube-system"
	}

	return opts
}

func (options *Options) AddFlags(fs *pflag.FlagSet) {
	logsv1.AddFlags(options.Logs, fs)

	fs.StringVar(&options.KubeConfigPath, "kubeconfig", options.KubeConfigPath, "Kubeconfig file for the local cluster.")
	fs.StringVar(&options.LeaseLockName, "lease-name", options.LeaseLockName, "Name of lease lock")
	fs.StringVar(&options.LeaseLockNamespace, "lease-namespace", options.LeaseLockNamespace, "Name of lease lock namespace")
	fs.StringVar(&options.ServerAddr, "server-address", options.ServerAddr, "Address for server")
	fs.DurationVar(&options.ProviderPollingInterval, "provider-polling-interval", 15*time.Second, "Interval for polling provider for updates")
}

func (options *Options) Complete() (*CompletedOptions, error) {
	if options.LeaseLockIdentity == "" {
		options.LeaseLockIdentity = fmt.Sprintf("%d", rand.Int31()) //nolint:gosec
	}

	return &CompletedOptions{
		completedOptions: &completedOptions{
			Logs:         options.Logs,
			ExtraOptions: options.ExtraOptions,
		},
	}, nil
}

func (options *CompletedOptions) Validate() error {
	return nil
}
