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

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/kube-bind/kube-bind/pkg/kubectl/bind/plugin"
)

var (
	bindExampleUses = `
	# select a kube-bind.io compatible service from the given URL, e.g. an API service.
	%[1]s bind https://mangodb.com/exports

	# authenticate and configure the services to bind, but don't actually bind them.
	%[1]s bind https://mangodb.com/exports --dry-run -o yaml > apiservice-binding-request.yaml

	# bind to to remote API service as configured above and actually bind to it. 
	%[1]s bind apiservice --token-file filename -f apiservice-binding-request.yaml

	# bind to the given remote API service from a https source.
	%[1]s bind apiservice --token-file token https://some-url/apiservice-binding-request.yaml
    `
)

func New(streams genericclioptions.IOStreams) (*cobra.Command, error) {
	bindOpts := plugin.NewBindOptions(streams)
	cmd := &cobra.Command{
		Use:          "bind",
		Short:        "Bind different remote types into the current cluster.",
		Example:      fmt.Sprintf(bindExampleUses, "kubectl"),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if err := bindOpts.Complete(args); err != nil {
				return err
			}

			if err := bindOpts.Validate(); err != nil {
				return err
			}

			return bindOpts.Run(cmd.Context())
		},
	}

	return cmd, nil
}
