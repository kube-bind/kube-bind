/*
Copyright 2024 The Kube Bind Authors.

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
	goflags "flag"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/component-base/version"
	"k8s.io/klog/v2"

	apiservicecmd "github.com/kube-bind/kube-bind/cli/pkg/kubectl/bind-apiservice/cmd"
	collectionscmd "github.com/kube-bind/kube-bind/cli/pkg/kubectl/bind-collections/cmd"
	logincmd "github.com/kube-bind/kube-bind/cli/pkg/kubectl/bind-login/cmd"
	templatescmd "github.com/kube-bind/kube-bind/cli/pkg/kubectl/bind-templates/cmd"
	bindcmd "github.com/kube-bind/kube-bind/cli/pkg/kubectl/bind/cmd"
	devcmd "github.com/kube-bind/kube-bind/cli/pkg/kubectl/dev/cmd"
)

func KubectlBindCommand() *cobra.Command {
	rootCmd, err := bindcmd.New(genericclioptions.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v", err)
		os.Exit(1)
	}
	// setup klog
	fs := goflags.NewFlagSet("klog", goflags.PanicOnError)
	klog.InitFlags(fs)
	rootCmd.PersistentFlags().AddGoFlagSet(fs)

	if v := version.Get().String(); len(v) == 0 {
		rootCmd.Version = "<unknown>"
	} else {
		rootCmd.Version = v
	}

	apiserviceCmd, err := apiservicecmd.New(genericclioptions.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v", err)
		os.Exit(1)
	}
	rootCmd.AddCommand(apiserviceCmd)

	loginCmd, err := logincmd.New(genericclioptions.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v", err)
		os.Exit(1)
	}
	rootCmd.AddCommand(loginCmd)

	templatesCmd, err := templatescmd.New(genericclioptions.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v", err)
		os.Exit(1)
	}
	rootCmd.AddCommand(templatesCmd)

	collectionsCmd, err := collectionscmd.New(genericclioptions.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v", err)
		os.Exit(1)
	}
	rootCmd.AddCommand(collectionsCmd)

	devCmd, err := devcmd.New(genericclioptions.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v", err)
		os.Exit(1)
	}
	rootCmd.AddCommand(devCmd)

	return rootCmd
}
