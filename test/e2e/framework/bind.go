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
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	"k8s.io/cli-runtime/pkg/genericclioptions"

	bindplugin "github.com/kube-bind/kube-bind/pkg/kubectl/bind/plugin"
)

func Bind(t *testing.T, authURLCh chan<- string, url string, args ...string) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	t.Logf("kubectl bind %s %s", url, strings.Join(args, " "))

	opts := bindplugin.NewBindOptions(genericclioptions.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr})
	cmd := &cobra.Command{}
	opts.AddCmdFlags(cmd)
	err := cmd.Flags().Parse(args)
	require.NoError(t, err)

	err = opts.Complete([]string{url})
	require.NoError(t, err)
	err = opts.Validate()
	require.NoError(t, err)
	err = opts.Run(ctx, authURLCh)
	require.NoError(t, err)
}
