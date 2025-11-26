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
	"testing"

	"github.com/stretchr/testify/require"

	bindcmd "github.com/kube-bind/kube-bind/cli/pkg/kubectl/bind/cmd"
)

func TestKubectlBindCommand(t *testing.T) {
	rootCmd := KubectlBindCommand()

	require.Equal(t, "kubectl-bind", rootCmd.Use, "Unexpected one-line command description")
	require.Equal(t, "kubectl plugin for kube-bind, bind different remote types into the current cluster.", rootCmd.Short, "Unexpected short command description")
	require.Contains(t, rootCmd.Long, "To bind a remote service, use the 'kubectl bind' command.", "Unexpected long command")
	require.Equal(t, rootCmd.Example, fmt.Sprintf(bindcmd.BindExampleUses, "kubectl"), "Unexpected command Example")
}

func TestKubectlBindArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{"Valid URL", []string{"https://example.com"}, false},
		{"Invalid argument", []string{"invalid-arg"}, true},
		{"Mixed valid and invalid", []string{"https://example.com", "invalid-arg"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rootCmd := KubectlBindCommand()
			err := rootCmd.Args(rootCmd, tt.args)
			if tt.wantErr {
				require.Error(t, err, "Expected error but got none")
			} else {
				require.NoError(t, err, "Unexpected error")
			}
		})
	}
}
