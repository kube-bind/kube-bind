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
	_ "k8s.io/client-go/plugin/pkg/client/auth/exec"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
	logsv1 "k8s.io/component-base/logs/api/v1"

	"github.com/kube-bind/kube-bind/cli/pkg/crd2apiresourceschema/plugin"
)

var (
	CRD2APIResourceSchemaUses = ``
)

func New(streams genericclioptions.IOStreams) (*cobra.Command, error) {
	opts := plugin.NewCRD2APIResourceSchemaOptions(streams)
	cmd := &cobra.Command{
		Use:          "crd2apiresourceschema",
		Short:        "Create APIResourceSchema from provided CRD in the cluster.",
		Example:      fmt.Sprintf(CRD2APIResourceSchemaUses, "crd2apiresourceschema"),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := logsv1.ValidateAndApply(opts.Logs, nil); err != nil {
				return err
			}

			if len(args) > 1 {
				return cmd.Help()
			}
			if err := opts.Complete(args); err != nil {
				return err
			}

			return opts.Run(cmd.Context())
		},
	}
	opts.AddCmdFlags(cmd)

	return cmd, nil
}
