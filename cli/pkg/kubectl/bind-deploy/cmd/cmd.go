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

package cmd

import (
	"fmt"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	logsv1 "k8s.io/component-base/logs/api/v1"

	"github.com/kube-bind/kube-bind/cli/pkg/help"
	"github.com/kube-bind/kube-bind/cli/pkg/kubectl/bind-deploy/plugin"

	_ "k8s.io/client-go/plugin/pkg/client/auth/exec"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
)

var (
	bindDeployExampleUses = `
	# Deploy konnector from a BindingResourceResponse stored in a secret
	%[1]s deploy --secret kube-bind/my-binding-response

	# Deploy konnector from a BindingResourceResponse file
	%[1]s deploy --file binding-response.yaml

	# Deploy konnector reading from stdin
	%[1]s deploy --file -
	`
)

func New(streams genericclioptions.IOStreams) (*cobra.Command, error) {
	opts := plugin.NewBindDeployOptions(streams)
	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Deploy konnector from a BindingResourceResponse",
		Long: help.Doc(`
		Deploy the konnector and create APIServiceBindings from a BindingResourceResponse.

		This command is useful when you have obtained a BindingResourceResponse through
		the CRD-based flow (BindableResourcesRequest) and want to deploy the konnector
		to establish the connection.

		The BindingResourceResponse can be provided via:
		- A Kubernetes secret (--secret namespace/name)
		- A file (--file path/to/file.yaml)
		- Standard input (--file -)
		`),
		Example:      fmt.Sprintf(bindDeployExampleUses, "kubectl bind"),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := logsv1.ValidateAndApply(opts.Logs, nil); err != nil {
				return err
			}

			if err := opts.Complete(args); err != nil {
				return err
			}

			if err := opts.Validate(); err != nil {
				return err
			}

			if !opts.NoBanner {
				yellow := color.New(color.BgRed, color.FgBlack).SprintFunc()
				fmt.Fprintf(streams.ErrOut, "%s\n\n", yellow("DISCLAIMER: This is a prototype. It will change in incompatible ways at any time."))
			}

			if err := opts.Run(cmd.Context()); err != nil {
				fmt.Fprintf(streams.ErrOut, "Error: %v\n", err)
				return nil
			}
			return nil
		},
	}
	opts.AddCmdFlags(cmd)

	return cmd, nil
}
