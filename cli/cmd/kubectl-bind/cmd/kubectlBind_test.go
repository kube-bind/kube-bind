package cmd

import (
	"fmt"
	"testing"

	bindcmd "github.com/kube-bind/kube-bind/cli/pkg/kubectl/bind/cmd"

	"github.com/stretchr/testify/require"
)

func TestKubectlBindCommand(t *testing.T) {
	rootCmd := KubectlBindCommand()

	require.Equal(t, "kubectl-bind", rootCmd.Use, "Unexpected one-line command description")
	require.Equal(t, "kubectl plugin for Kube-Bind.io, bind different remote types into the current cluster.", rootCmd.Short, "Unexpected short command description")
	require.Contains(t, rootCmd.Long, "To bind a remote service, use the 'kubectl bind' command.", "Unexpected lond command Long")
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
