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
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	_ "k8s.io/client-go/plugin/pkg/client/auth/exec"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
	logsv1 "k8s.io/component-base/logs/api/v1"

	"github.com/kube-bind/kube-bind/cli/pkg/crd2apiresourceschema/plugin"
)

var (
	CRD2APIResourceSchemaUses = `
# Generate APIResourceSchemas from provided CRDs in the cluster and save them to YAML files in the specified output directory
crd2apiresourceschema --output-dir /output/dir

# Generate APIResourceSchemas from provided CRDs in the cluster and save them to YAML files, specifying a different kubeconfig and output directory
crd2apiresourceschema --kubeconfig /path/to/your/kubeconfig --output-dir /path/to/output/dir

# Generate and create APIResourceSchema objects for all CRDs in the cluster
crd2apiresourceschema --generate-in-cluster

# Generate and create APIResourceSchema objects for all CRDs in the cluster, specifying a different kubeconfig
crd2apiresourceschema --kubeconfig /path/to/your/kubeconfig --generate-in-cluster
`
)

func New(streams genericclioptions.IOStreams) (*cobra.Command, error) {
	opts := plugin.NewCRD2APIResourceSchemaOptions(streams)
	cmd := &cobra.Command{
		Use:          "crd2apiresourceschema",
		Short:        "Create APIResourceSchema from provided CRDs in the cluster.",
		Example:      CRD2APIResourceSchemaUses,
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

			if err := opts.Validate(); err != nil {
				return err
			}

			return opts.Run(cmd.Context())
		},
	}
	opts.AddCmdFlags(cmd)

	return cmd, nil
}
