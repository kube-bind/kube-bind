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

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/kube-bind/kube-bind/pkg/kubectl/bind/plugin"
)

var (
	// TODO: add other examples related to permission claim commands.
	bindExampleUses = `
    # select a kube-bind.io compatible service from the given URL, e.g. an API service.
	%[1]s bind https://mangodb.com/exports

	# authenticate and configure the services to bind, but don't actually bind them.
	%[1]s bind https://mangodb.com/exports --dry-run -o yaml > apiservice-binding-requests.yaml

	# bind to a remote API service as configured above and actually bind to it, e.g. in GitOps automation.
	%[1]s bind apiservice --remote-kubeconfig name -f apiservice-binding-requests.yaml

	# bind to a remote API service via a request manifest from a https URL.
	%[1]s bind apiservice --remote-kubeconfig name https://some-url.com/apiservice-binding-requests.yaml
	`
)

func New(streams genericclioptions.IOStreams) (*cobra.Command, error) {
	opts := plugin.NewBindOptions(streams)
	cmd := &cobra.Command{
		Use:          "bind",
		Short:        "Bind different remote types into the current cluster.",
		Example:      fmt.Sprintf(bindExampleUses, "kubectl"),
		SilenceUsage: true,
		Args: func(cmd *cobra.Command, args []string) error {
			// if no subcommand is specified, only a URL, this is fine.
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			yellow := color.New(color.BgRed, color.FgBlack).SprintFunc()
			fmt.Fprintf(streams.ErrOut, yellow("DISCLAIMER: This is a prototype. It will change in incompatible ways at any time.")+"\n\n") // nolint: errcheck

			if len(args) == 0 {
				return cmd.Help()
			}
			if err := opts.Complete(args); err != nil {
				return err
			}

			if err := opts.Validate(); err != nil {
				return err
			}

			return opts.Run(cmd.Context(), nil)
		},
	}
	opts.AddCmdFlags(cmd)

	return cmd, nil
}
