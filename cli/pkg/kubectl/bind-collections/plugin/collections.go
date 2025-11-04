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
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/util/duration"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/component-base/logs"
	logsv1 "k8s.io/component-base/logs/api/v1"

	"github.com/kube-bind/kube-bind/cli/pkg/kubectl/base"
	kubebindv1alpha2 "github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2"
)

// CollectionsOptions contains the options for listing collections
type CollectionsOptions struct {
	*base.Options
	Logs *logs.Options

	Print   *genericclioptions.PrintFlags
	printer printers.ResourcePrinter
}

// NewCollectionsOptions creates a new CollectionsOptions
func NewCollectionsOptions(streams genericclioptions.IOStreams) *CollectionsOptions {
	return &CollectionsOptions{
		Options: base.NewOptions(streams),
		Logs:    logs.NewOptions(),
		Print:   genericclioptions.NewPrintFlags("collections").WithDefaultOutput(""),
	}
}

// AddCmdFlags adds command line flags
func (o *CollectionsOptions) AddCmdFlags(cmd *cobra.Command) {
	o.Options.BindFlags(cmd)
	logsv1.AddFlags(o.Logs, cmd.Flags())
	o.Print.AddFlags(cmd)
}

// Complete completes the options
func (o *CollectionsOptions) Complete(args []string) error {
	// Set this before complete base settings as login accepts server URL as argument without flag.
	if len(args) > 0 {
		o.Options.Server = args[0]
	}

	if err := o.Options.Complete(false); err != nil {
		return err
	}

	printer, err := o.Print.ToPrinter()
	if err != nil {
		return err
	}
	o.printer = printer

	return nil
}

// Validate validates the options
func (o *CollectionsOptions) Validate() error {
	return o.Options.Validate()
}

// Run executes the collections listing command
func (o *CollectionsOptions) Run(ctx context.Context) error {
	// Get authenticated client
	client, err := o.Options.GetAuthenticatedClient()
	if err != nil {
		return fmt.Errorf("failed to create authenticated client: %w", err)
	}

	// Fetch collections
	collections, err := client.GetCollections(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch collections: %w", err)
	}

	return o.printCollections(collections)
}

// printCollections handles printing collections using the configured printer
func (o *CollectionsOptions) printCollections(collections *kubebindv1alpha2.CollectionList) error {
	// For non-table output formats, print the whole list
	if o.Print.OutputFormat != nil && *o.Print.OutputFormat != "" {
		collections.SetGroupVersionKind(kubebindv1alpha2.SchemeGroupVersion.WithKind("CollectionList"))
		return o.printer.PrintObj(collections, o.Options.IOStreams.Out)
	}

	// For table output, create a custom human-readable table
	return o.printCollectionsTable(collections)
}

// printCollectionsTable prints collections in table format using tabwriter
// TODO: Replace with custom TablePrinter when available in cli-runtime
func (o *CollectionsOptions) printCollectionsTable(collections *kubebindv1alpha2.CollectionList) error {
	w := tabwriter.NewWriter(o.Options.IOStreams.Out, 0, 0, 3, ' ', 0)
	fmt.Fprintf(w, "NAME\tDESCRIPTION\tTEMPLATES\tAGE\n")

	for _, item := range collections.Items {
		description := item.Spec.Description
		if description == "" {
			description = "<none>"
		}

		// Create a comma-separated list of template names
		templateNames := make([]string, len(item.Spec.Templates))
		for i, template := range item.Spec.Templates {
			templateNames[i] = template.Name
		}
		templates := strings.Join(templateNames, ",")

		age := duration.HumanDuration(time.Since(item.CreationTimestamp.Time))

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", item.Name, description, templates, age)
	}

	return w.Flush()
}
