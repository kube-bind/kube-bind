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
	"strings"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	logsv1 "k8s.io/component-base/logs/api/v1"

	"github.com/kube-bind/kube-bind/cli/pkg/kubectl/bind-templates/plugin"
)

var (
	TemplatesExampleUses = `
	# List templates from currently authenticated server
	%[1]s templates

	# List templates from specific server 
	%[1]s templates

	# List templates using --server flag to override current server
	%[1]s templates --server https://mangodb.com

	# List templates with JSON/YAML output
	%[1]s templates -o json
	%[1]s templates -o yaml
	`
)

func New(streams genericclioptions.IOStreams) (*cobra.Command, error) {
	opts := plugin.NewTemplatesOptions(streams)
	cmd := &cobra.Command{
		Use:   "templates [server-url]",
		Short: "List available exported templates from a kube-bind server",
		Long: `List all available exported templates from a kube-bind server.

This command connects to a kube-bind server and displays all the templates
that are available for binding. By default, it uses the current authenticated
server from your configuration.

If you haven't authenticated to any server yet, you must provide a server URL
argument or use 'kubectl bind-login <server>' first.`,
		Example:      fmt.Sprintf(TemplatesExampleUses, "kubectl"),
		SilenceUsage: true,
		Args: func(cmd *cobra.Command, args []string) error {
			for _, arg := range args {
				if !strings.HasPrefix(arg, "http://") && !strings.HasPrefix(arg, "https://") {
					return fmt.Errorf("invalid server URL: %s", arg)
				}
			}
			return nil
		},
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

			return opts.Run(cmd.Context())
		},
	}
	opts.AddCmdFlags(cmd)

	return cmd, nil
}
