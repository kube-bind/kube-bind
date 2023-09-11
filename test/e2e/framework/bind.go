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
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	"k8s.io/cli-runtime/pkg/genericclioptions"

	bindapiserviceplugin "github.com/kube-bind/kube-bind/pkg/kubectl/bind-apiservice/plugin"
	bindplugin "github.com/kube-bind/kube-bind/pkg/kubectl/bind/plugin"
)

func Bind(t *testing.T, iostreams genericclioptions.IOStreams, authURLCh chan<- string, invocations chan<- SubCommandInvocation, positionalArg string, flags ...string) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	args := flags
	if positionalArg != "" {
		args = append(args, positionalArg)
	}
	t.Logf("kubectl bind %s", strings.Join(args, " "))

	opts := bindplugin.NewBindOptions(iostreams)
	cmd := &cobra.Command{}
	opts.AddCmdFlags(cmd)
	err := cmd.Flags().Parse(flags)
	require.NoError(t, err)

	err = opts.Complete([]string{positionalArg})
	require.NoError(t, err)
	err = opts.Validate()
	require.NoError(t, err)

	opts.Runner = func(cmd *exec.Cmd) error {
		bs, err := io.ReadAll(cmd.Stdin)
		if err != nil {
			return err
		}
		if invocations != nil {
			invocations <- SubCommandInvocation{
				Executable: cmd.Args[0],
				Args:       cmd.Args[1:],
				Stdin:      bs,
			}
		}
		t.Logf("Running command: %s\nstdin:\n", cmd.String())
		t.Logf("%s", bs)

		return nil
	}
	err = opts.Run(ctx, authURLCh)
	require.NoError(t, err)
}

type SubCommandInvocation struct {
	Executable string
	Args       []string
	Stdin      []byte
}

func BindAPIService(t *testing.T, Stdin io.Reader, positionalArg string, flags ...string) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	args := flags
	if positionalArg != "" {
		args = append(args, positionalArg)
	}
	t.Logf("kubectl bind apiservice %s", strings.Join(args, " "))

	opts := bindapiserviceplugin.NewBindAPIServiceOptions(genericclioptions.IOStreams{In: Stdin, Out: os.Stdout, ErrOut: os.Stderr})
	cmd := &cobra.Command{}
	opts.AddCmdFlags(cmd)
	err := cmd.Flags().Parse(flags)
	require.NoError(t, err)

	err = opts.Complete([]string{positionalArg})
	require.NoError(t, err)
	err = opts.Validate()
	require.NoError(t, err)
	err = opts.Run(ctx)
	require.NoError(t, err)
}
