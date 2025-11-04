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

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	logsv1 "k8s.io/component-base/logs/api/v1"

	"github.com/kube-bind/kube-bind/cli/pkg/help"
	"github.com/kube-bind/kube-bind/cli/pkg/kubectl/bind-login/plugin"
)

var (
	LoginExampleUses = `
	# Login to a kube-bind server
	%[1]s login https://my-kube-bind-server.example.com

	# Login with a custom callback port
	%[1]s login https://my-kube-bind-server.example.com --callback-port 8081

	# Login and show authentication status
	%[1]s login https://my-kube-bind-server.example.com --show-token
	`
)

func New(streams genericclioptions.IOStreams) (*cobra.Command, error) {
	opts := plugin.NewLoginOptions(streams)
	cmd := &cobra.Command{
		Use:   "login [SERVER_URL]",
		Short: "Login to a kube-bind server and store authentication credentials",
		Long: help.Doc(`
		Login to a kube-bind server using OAuth2 authentication flow.
		
		The command will open your browser to complete the OAuth2 flow and
		store the resulting JWT token in ~/.kube-bind/config for use by
		subsequent commands.
		
		The SERVER_URL should point to the root of your kube-bind server,
		e.g. https://my-server.example.com
	`),
		Example:      fmt.Sprintf(LoginExampleUses, "kubectl bind"),
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
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
