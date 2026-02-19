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

package plugin

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/component-base/logs"
	logsv1 "k8s.io/component-base/logs/api/v1"

	"github.com/kube-bind/kube-bind/cli/pkg/kubectl/base"
)

const (
	defaultIdentityNamespace = "kube-system"
)

// ClusterIdentityOptions contains the options for getting the cluster identity
type ClusterIdentityOptions struct {
	genericclioptions.IOStreams
	Logs *logs.Options

	// Kubeconfig specifies kubeconfig file(s).
	Kubeconfig string
	// KubectlOverrides stores the extra client connection fields, such as context, user, etc.
	KubectlOverrides *clientcmd.ConfigOverrides
	// Namespace is the namespace to use for deriving the cluster identity
	Namespace string
	// ClientConfig is the resolved clientcmd.ClientConfig based on the client connection flags.
	ClientConfig clientcmd.ClientConfig
}

// NewClusterIdentityOptions creates a new ClusterIdentityOptions
func NewClusterIdentityOptions(streams genericclioptions.IOStreams) *ClusterIdentityOptions {
	return &ClusterIdentityOptions{
		IOStreams:        streams,
		Logs:             logs.NewOptions(),
		KubectlOverrides: &clientcmd.ConfigOverrides{},
		Namespace:        defaultIdentityNamespace,
	}
}

// AddCmdFlags adds command line flags
func (o *ClusterIdentityOptions) AddCmdFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&o.Kubeconfig, "kubeconfig", o.Kubeconfig, "Path to the kubeconfig file")
	cmd.Flags().StringVarP(&o.Namespace, "namespace", "n", o.Namespace, "Namespace to use for deriving the cluster identity (default: kube-system)")

	// Add context flag from kubectl overrides
	kubectlConfigOverrideFlags := clientcmd.RecommendedConfigOverrideFlags("")
	kubectlConfigOverrideFlags.AuthOverrideFlags.ClientCertificate.LongName = ""
	kubectlConfigOverrideFlags.AuthOverrideFlags.ClientKey.LongName = ""
	kubectlConfigOverrideFlags.AuthOverrideFlags.Impersonate.LongName = ""
	kubectlConfigOverrideFlags.AuthOverrideFlags.ImpersonateGroups.LongName = ""
	kubectlConfigOverrideFlags.ContextOverrideFlags.ClusterName.LongName = ""
	kubectlConfigOverrideFlags.Timeout.LongName = ""

	clientcmd.BindOverrideFlags(o.KubectlOverrides, cmd.PersistentFlags(), kubectlConfigOverrideFlags)

	logsv1.AddFlags(o.Logs, cmd.Flags())
}

// Complete completes the options
func (o *ClusterIdentityOptions) Complete(args []string) error {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	loadingRules.ExplicitPath = o.Kubeconfig

	startingConfig, err := loadingRules.GetStartingConfig()
	if err != nil {
		return err
	}

	o.ClientConfig = clientcmd.NewDefaultClientConfig(*startingConfig, o.KubectlOverrides)

	return nil
}

// Validate validates the options
func (o *ClusterIdentityOptions) Validate() error {
	if o.Namespace == "" {
		return fmt.Errorf("namespace cannot be empty")
	}
	return nil
}

// Run executes the cluster-identity command
func (o *ClusterIdentityOptions) Run(ctx context.Context) error {
	restConfig, err := o.ClientConfig.ClientConfig()
	if err != nil {
		return fmt.Errorf("failed to get REST config: %w", err)
	}

	identity, err := base.GetClusterIdentityFromNamespace(ctx, restConfig, o.Namespace)
	if err != nil {
		return fmt.Errorf("failed to get cluster identity: %w", err)
	}

	fmt.Fprintln(o.Out, identity)

	return nil
}
