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

	"github.com/kube-bind/kube-bind/pkg/kubectl/bind-apiservice/plugin"
)

var (
	bindAPIServiceExampleUses = `
	# bind to a remote API service. Use kubectl bind to create the APIBindingRequest interactively. 
	%[1]s apiservice --remote-kubeconfig file -f apiservice-binding-request.yaml

	# bind to a remote API service via a request manifest from a https URL.
	%[1]s apiservice --remote-kubeconfig file https://some-url.com/apiservice-binding-requests.yaml

    # bind to a API service directly without any remote agent or service provider.
	%[1]s apiservice --remote-kubeconfig file -n remote-namespace resources.group/v1
	`
)

func New(streams genericclioptions.IOStreams) (*cobra.Command, error) {
	opts := plugin.NewBindAPIServiceOptions(streams)
	cmd := &cobra.Command{
		Use:          "apiservice https://<url-to-a-APIBindingRequest>|-f <file-to-a-APIBindingRequest>",
		Short:        "Bind to a remote API service",
		Example:      fmt.Sprintf(bindAPIServiceExampleUses, "kubectl bind"),
		SilenceUsage: true,
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

			return opts.Run(cmd.Context())
		},
	}
	opts.BindFlags(cmd)

	return cmd, nil
}
