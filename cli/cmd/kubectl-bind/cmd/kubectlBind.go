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

	"github.com/kube-bind/kube-bind/cli/pkg/help"
	apiservicecmd "github.com/kube-bind/kube-bind/cli/pkg/kubectl/bind-apiservice/cmd"
	bindcmd "github.com/kube-bind/kube-bind/cli/pkg/kubectl/bind/cmd"
)

func KubectlBindCommand() *cobra.Command {
	root := &cobra.Command{
		Use:   "bind",
		Short: "kubectl plugin for Kube-Bind.io",
		Long: help.Doc(`
			kube-bind is a project that aims to provide better support for 
			service providers and consumers that reside in distinct Kubernetes clusters.

			For more information, see: https://kube-bind.io
		`),
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	// setup klog
	fs := goflags.NewFlagSet("klog", goflags.PanicOnError)
	klog.InitFlags(fs)
	root.PersistentFlags().AddGoFlagSet(fs)

	if v := version.Get().String(); len(v) == 0 {
		root.Version = "<unknown>"
	} else {
		root.Version = v
	}

	bindCmd, err := bindcmd.New(genericclioptions.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v", err)
		os.Exit(1)
	}

	apiserviceCmd, err := apiservicecmd.New(genericclioptions.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v", err)
		os.Exit(1)
	}
	bindCmd.AddCommand(apiserviceCmd)

	return bindCmd
}
