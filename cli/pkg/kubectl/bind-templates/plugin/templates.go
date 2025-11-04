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

// TemplatesOptions contains the options for listing exported templates
type TemplatesOptions struct {
	*base.Options
	Logs *logs.Options

	Print   *genericclioptions.PrintFlags
	printer printers.ResourcePrinter
}

// ExportedResource represents a resource available for binding
type ExportedResource struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Resources   []string `json:"resources,omitempty"`
}

// NewTemplatesOptions creates a new TemplatesOptions
func NewTemplatesOptions(streams genericclioptions.IOStreams) *TemplatesOptions {
	return &TemplatesOptions{
		Options: base.NewOptions(streams),
		Logs:    logs.NewOptions(),
		Print:   genericclioptions.NewPrintFlags("templates").WithDefaultOutput(""),
	}
}

// AddCmdFlags adds command line flags
func (o *TemplatesOptions) AddCmdFlags(cmd *cobra.Command) {
	o.Options.BindFlags(cmd)
	logsv1.AddFlags(o.Logs, cmd.Flags())
	o.Print.AddFlags(cmd)
}

// Complete completes the options
func (o *TemplatesOptions) Complete(args []string) error {
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
func (o *TemplatesOptions) Validate() error {
	return nil
}

// Run executes the templates listing command
func (o *TemplatesOptions) Run(ctx context.Context) error {
	// Get authenticated client
	client, err := o.Options.GetAuthenticatedClient()
	if err != nil {
		return fmt.Errorf("failed to create authenticated client: %w", err)
	}

	// Fetch exported templates
	templates, err := client.GetTemplates(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch exported templates: %w", err)
	}

	// Display templates
	return o.displayTemplates(templates)
}

// displayTemplates displays the templates using the configured printer
func (o *TemplatesOptions) displayTemplates(templates *kubebindv1alpha2.APIServiceExportTemplateList) error {
	// For non-table output formats, print the whole list
	if o.Print.OutputFormat != nil && *o.Print.OutputFormat != "" {
		templates.SetGroupVersionKind(kubebindv1alpha2.SchemeGroupVersion.WithKind("APIServiceExportTemplateList"))
		return o.printer.PrintObj(templates, o.IOStreams.Out)
	}

	// For table output, create a custom human-readable table
	return o.displayTemplatesTable(templates)
}

// displayTemplatesTable prints templates in table format using tabwriter
// TODO: Replace with custom TablePrinter when available in cli-runtime
func (o *TemplatesOptions) displayTemplatesTable(templates *kubebindv1alpha2.APIServiceExportTemplateList) error {
	w := tabwriter.NewWriter(o.Options.IOStreams.Out, 0, 0, 3, ' ', 0)
	fmt.Fprintf(w, "NAME\tRESOURCES\tPERMISSIONCLAIMS\tAGE\n")

	for _, item := range templates.Items {
		// Create a comma-separated list of resource groups
		resourceGroups := make([]string, len(item.Spec.Resources))
		for i, resource := range item.Spec.Resources {
			resourceGroups[i] = resource.Group
		}
		resources := strings.Join(resourceGroups, ",")

		// Create a comma-separated list of permission claim resources
		permissionResources := make([]string, len(item.Spec.PermissionClaims))
		for i, claim := range item.Spec.PermissionClaims {
			permissionResources[i] = claim.Resource
		}
		permissionClaims := strings.Join(permissionResources, ",")

		age := duration.HumanDuration(time.Since(item.CreationTimestamp.Time))

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", item.Name, resources, permissionClaims, age)
	}

	return w.Flush()
}
