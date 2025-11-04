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

package framework

import (
	"context"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/kube-bind/kube-bind/cli/pkg/client"
	"github.com/kube-bind/kube-bind/cli/pkg/config"
	loginplugin "github.com/kube-bind/kube-bind/cli/pkg/kubectl/bind-login/plugin"
)

func GetKubeBindRestClient(t *testing.T, configFile string) client.Client {
	t.Helper()

	c, err := config.LoadConfigFromFile(configFile)
	require.NoError(t, err)

	server, _, err := c.GetCurrentServer()
	require.NoError(t, err)
	require.NotNil(t, server, "no current server configured in %s", configFile)

	client, err := client.NewClient(*server, client.WithInsecure(true))
	require.NoError(t, err)

	return client
}

func Login(t *testing.T, iostreams genericclioptions.IOStreams, authURLCh chan<- string, configFile, serverURL, clusterID string) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	t.Logf("kubectl login %s --skip-browser --config-file=%s --cluster=%s", serverURL, configFile, clusterID)

	opts := loginplugin.NewLoginOptions(iostreams)
	cmd := &cobra.Command{}
	opts.AddCmdFlags(cmd)
	args := []string{serverURL}
	if clusterID != "" {
		args = append(args, "--cluster="+clusterID)
	}
	if configFile != "" {
		args = append(args, "--config-file="+configFile)
	}
	err := cmd.Flags().Parse(args)
	require.NoError(t, err)

	err = opts.Complete(args)
	require.NoError(t, err)

	err = opts.Validate()
	require.NoError(t, err)
	err = opts.Run(ctx, authURLCh)
	require.NoError(t, err)
}
