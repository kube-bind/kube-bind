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
	"github.com/spf13/pflag"
)

type Options struct {
	ExtraOptions
}

type ExtraOptions struct {
	APIExportEndpointSliceName string
}

type completedOptions struct {
	ExtraOptions
}

type CompletedOptions struct {
	*completedOptions
}

func NewOptions() *Options {
	return &Options{
		ExtraOptions: ExtraOptions{
			APIExportEndpointSliceName: "kube-bind.io",
		},
	}
}

func (options *Options) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&options.ExtraOptions.APIExportEndpointSliceName, "api-export-endpoint-slice-name", options.ExtraOptions.APIExportEndpointSliceName, "name of the APIExport EndpointSlice to watch")
}

func (options *Options) Complete() (*CompletedOptions, error) {
	return &CompletedOptions{
		completedOptions: &completedOptions{
			ExtraOptions: options.ExtraOptions,
		},
	}, nil
}

func (options *CompletedOptions) Validate() error {
	return nil
}
