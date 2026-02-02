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

	"github.com/kube-bind/kube-bind/cli/pkg/kubectl/cluster-identity/plugin"
)

var (
	ClusterIdentityExampleUses = `
	# Get the cluster identity for the current kubeconfig context
	%[1]s cluster-identity

	# Get the cluster identity using a specific kubeconfig
	%[1]s cluster-identity --kubeconfig /path/to/kubeconfig

	# Get the cluster identity using a specific context
	%[1]s cluster-identity --context my-context

	# Get the cluster identity from a specific namespace (default: kube-system)
	%[1]s cluster-identity --namespace my-namespace
	`
)

func New(streams genericclioptions.IOStreams) (*cobra.Command, error) {
	opts := plugin.NewClusterIdentityOptions(streams)
	cmd := &cobra.Command{
		Use:   "cluster-identity",
		Short: "Get the cluster identity for the current Kubernetes cluster",
		Long: `Get the cluster identity for the current Kubernetes cluster.

The cluster identity is derived from the UID of a namespace (by default kube-system).
This identity is used by kube-bind to uniquely identify consumer clusters when
binding to service providers.

This is a helper command for users who are not using the kube-bind web server
and need to manually provide the cluster identity.`,
		Example:      fmt.Sprintf(ClusterIdentityExampleUses, "kubectl bind"),
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

			return opts.Run(cmd.Context())
		},
	}
	opts.AddCmdFlags(cmd)

	return cmd, nil
}
