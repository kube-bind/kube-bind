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
	"strings"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	logsv1 "k8s.io/component-base/logs/api/v1"

	"github.com/kube-bind/kube-bind/cli/pkg/help"
	"github.com/kube-bind/kube-bind/cli/pkg/kubectl/bind/plugin"

	_ "k8s.io/client-go/plugin/pkg/client/auth/exec"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
)

var (
	BindExampleUses = `
	# %[1]s bind login --server https://my-kube-bind-server.com

	# Open kube-bind UI for current server context 
	%[1]s bind

	# List available templates (CLI mode)
	%[1]s bind templates

	# Bind specific template (CLI mode)
	%[1]s bind apiservice --name my-api --template-name database-service

	# View template details without binding
	%[1]s bind apiservice --name my-api --template-name database-service --dry-run
	`
)

func New(streams genericclioptions.IOStreams) (*cobra.Command, error) {
	opts := plugin.NewBindOptions(streams)
	cmd := &cobra.Command{
		Use:   "bind [server-url]",
		Short: "Open kube-bind UI or bind templates from a remote server.",
		Long: help.Doc(`
		kube-bind allows you to bind remote services into your cluster using either
		a web UI or command-line interface.

		By default, 'kubectl bind' opens the kube-bind web UI in your browser.

		For more information, see: https://docs.kube-bind.io
	`),
		Example:      fmt.Sprintf(BindExampleUses, "kubectl"),
		SilenceUsage: true,
		Args: func(cmd *cobra.Command, args []string) error {
			// Allow 0 or 1 arguments
			if len(args) > 1 {
				return fmt.Errorf("too many arguments, expected at most 1")
			}
			// If argument is provided, it must be a URL
			if len(args) == 1 {
				arg := args[0]
				if !strings.HasPrefix(arg, "http://") && !strings.HasPrefix(arg, "https://") {
					return fmt.Errorf("server URL must start with http:// or https://, got: %s", arg)
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

			return opts.Run(cmd.Context(), nil)
		},
	}
	opts.AddCmdFlags(cmd)

	return cmd, nil
}
