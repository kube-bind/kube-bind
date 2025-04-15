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

	"github.com/spf13/cobra"

	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/component-base/logs"
	logsv1 "k8s.io/component-base/logs/api/v1"

	"github.com/kube-bind/kube-bind/cli/pkg/kubectl/base"
)

// CRD2APIResourceSchemaOptions are the options for the kubectl-bind-apiservice command.
type CRD2APIResourceSchemaOptions struct {
	Options *base.Options
	Logs    *logs.Options
	Print   *genericclioptions.PrintFlags
}

// NewCRD2APIResourceSchemaOptions returns new BindAPIServiceOptions.
func NewCRD2APIResourceSchemaOptions(streams genericclioptions.IOStreams) *CRD2APIResourceSchemaOptions {
	return &CRD2APIResourceSchemaOptions{
		Options: base.NewOptions(streams),
		Logs:    logs.NewOptions(),
		Print:   genericclioptions.NewPrintFlags("kubectl-bind-apiservice"),
	}
}

// AddCmdFlags binds fields to cmd's flagset.
func (b *CRD2APIResourceSchemaOptions) AddCmdFlags(cmd *cobra.Command) {
	b.Options.BindFlags(cmd)
	logsv1.AddFlags(b.Logs, cmd.Flags())
	b.Print.AddFlags(cmd)
}

// Complete ensures all fields are initialized.
func (b *CRD2APIResourceSchemaOptions) Complete(args []string) error {
	if err := b.Options.Complete(); err != nil {
		return err
	}

	return nil
}

// Run starts the binding process.
func (b *CRD2APIResourceSchemaOptions) Run(ctx context.Context) error {

	return nil
}
