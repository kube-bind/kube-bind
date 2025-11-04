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

package base

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/kube-bind/kube-bind/cli/pkg/config"
)

// Options contains options common to most CLI plugins.
type Options struct {
	genericclioptions.IOStreams
	// OptOutOfDefaultKubectlFlags indicates that the standard kubectl/kubeconfig-related flags should not be bound
	// by default.
	OptOutOfDefaultKubectlFlags bool
	// Kubeconfig specifies kubeconfig file(s).
	Kubeconfig string
	// KubectlOverrides stores the extra client connection fields, such as context, user, etc.
	KubectlOverrides *clientcmd.ConfigOverrides
	// Server is the kube-bind server name to use (overrides kube-bind config current server)
	Server string
	// Cluster is the kube-bind cluster name to use (overrides kube-bind config current cluster)
	Cluster string
	// SkipInsecure skips TLS verification (for development)
	SkipInsecure bool
	// ConfigFile is the path to the kube-bind configuration file
	ConfigFile string
	// ClientConfig is the resolved cliendcmd.ClientConfig based on the client connection flags. This is only valid
	// after calling Complete.
	ClientConfig clientcmd.ClientConfig

	// config is config struct. Used only for login process.
	config *config.Config
	// Server is the loaded kube-bind server used by the plugins.
	server *config.Server
}

// NewOptions provides an instance of Options with default values.
func NewOptions(streams genericclioptions.IOStreams) *Options {
	return &Options{
		KubectlOverrides: &clientcmd.ConfigOverrides{},
		IOStreams:        streams,
	}
}

// BindFlags binds options fields to cmd's flagset.
func (o *Options) BindFlags(cmd *cobra.Command) {
	if o.OptOutOfDefaultKubectlFlags {
		return
	}

	cmd.Flags().StringVar(&o.Kubeconfig, "kubeconfig", o.Kubeconfig, "path to the kubeconfig file")

	// We add only a subset of kubeconfig-related flags to the plugin.
	// All those with LongName == "" will be ignored.
	kubectlConfigOverrideFlags := clientcmd.RecommendedConfigOverrideFlags("")
	kubectlConfigOverrideFlags.AuthOverrideFlags.ClientCertificate.LongName = ""
	kubectlConfigOverrideFlags.AuthOverrideFlags.ClientKey.LongName = ""
	kubectlConfigOverrideFlags.AuthOverrideFlags.Impersonate.LongName = ""
	kubectlConfigOverrideFlags.AuthOverrideFlags.ImpersonateGroups.LongName = ""
	kubectlConfigOverrideFlags.ContextOverrideFlags.ClusterName.LongName = ""
	kubectlConfigOverrideFlags.Timeout.LongName = ""

	clientcmd.BindOverrideFlags(o.KubectlOverrides, cmd.PersistentFlags(), kubectlConfigOverrideFlags)

	// Add common kube-bind flags
	cmd.Flags().StringVar(&o.Server, "server", o.Server, "The kube-bind server name to use (overrides kube-bind config current server)")
	cmd.Flags().StringVar(&o.Cluster, "cluster", o.Cluster, "The kube-bind cluster name to use (overrides kube-bind config current cluster)")
	cmd.Flags().BoolVar(&o.SkipInsecure, "insecure-skip-tls-verify", false, "Skip TLS certificate verification (not recommended)")
	cmd.Flags().StringVar(&o.ConfigFile, "config-file", "", "Path to the kube-bind configuration file")
}

// Complete initializes ClientConfig based on Kubeconfig and KubectlOverrides, and resolves server configuration.
func (o *Options) Complete(skipValidate bool) error {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	loadingRules.ExplicitPath = o.Kubeconfig

	startingConfig, err := loadingRules.GetStartingConfig()
	if err != nil {
		return err
	}

	o.ClientConfig = clientcmd.NewDefaultClientConfig(*startingConfig, o.KubectlOverrides)

	if o.ConfigFile == "" {
		o.ConfigFile = config.GetDefaultConfigFilePath()
	}

	// Create directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(o.ConfigFile), 0700); err != nil {
		return fmt.Errorf("unable to create config directory: %w", err)
	}

	if _, err := os.Stat(o.ConfigFile); os.IsNotExist(err) {
		// Create empty config file
		emptyConfig := config.NewConfig(o.ConfigFile)
		if err := emptyConfig.SaveConfig(); err != nil {
			return fmt.Errorf("unable to create config file: %w", err)
		}
	}

	switch {
	case o.Server != "" && o.Cluster != "":
		// Both server and cluster specified separately - do nothing
	case o.Server != "" && strings.Contains(o.Server, "@"):
		// Server specified in server@cluster format
		parts := strings.SplitN(o.Server, "@", 2)
		o.Server = parts[0]
		o.Cluster = parts[1]
	case o.Server != "" && o.Cluster == "":
		// Only server specified - do nothing. Cluster is empty
	case o.Server == "":
		// No server specified - do nothing. Will be resolved from config
	}

	// If server name is not specified, use current server from config.
	// Server should always be set either with cluster or without.
	// So if its empty here, we need to resolve it.
	var s *config.Server
	var c *config.Config
	if o.Server == "" {
		c, err = config.LoadConfigFromFile(o.ConfigFile)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		s, _, err = c.GetCurrentServer()
		if err != nil && !skipValidate {
			return fmt.Errorf("no current server configured. Use 'kubectl bind-login <server> {--cluster <cluster>}' to login first: %w", err)
		}
	} else {
		// Validate that the specified server exists in config
		c, err = config.LoadConfigFromFile(o.ConfigFile)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		var exists bool
		s, exists = c.Get(o.Server, o.Cluster)
		if !exists && !skipValidate {
			err := fmt.Errorf("server %q not found in config", o.Server)
			if o.Cluster != "" {
				err = fmt.Errorf("server %q with cluster %q not found in config", o.Server, o.Cluster)
			}
			return err
		}
	}
	if (s == nil && !skipValidate) || c == nil {
		// This should never happen. This is programming error.
		return fmt.Errorf("failed to resolve server configuration")
	}

	if s != nil {
		o.Server = s.URL
		o.Cluster = s.Cluster
	}
	o.server = s
	o.config = c

	return nil
}

// Validate validates the configured options.
func (o *Options) Validate() error {
	if o.Server != "" && strings.Contains(o.Server, "@") && o.Cluster != "" {
		return fmt.Errorf("cannot specify both server in 'server@cluster' format and --cluster flag")
	}

	if o.Cluster != "" && o.Server == "" {
		return fmt.Errorf("cannot specify --cluster without --server")
	}
	if _, err := url.Parse(o.Server); err != nil {
		return fmt.Errorf("invalid server URL: %w", err)
	}

	return nil
}

func (o *Options) GetConfig() *config.Config {
	return o.config
}
