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

package plugin

import (
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/printers"
)

// AllowedFormats is the list of formats in which data can be displayed
func (b *BindOptions) AllowedFormats() []string {
	formats := b.JSONYamlPrintFlags.AllowedFormats()
	return formats
}

// ToPrinter attempts to find a composed set of PrintFlags suitable for
// returning a printer based on current flag values.
func (b *BindOptions) ToPrinter() (printers.ResourcePrinter, error) {
	if p, err := b.JSONYamlPrintFlags.ToPrinter(b.OutputFormat); !genericclioptions.IsNoCompatiblePrinterError(err) {
		return p, err
	}

	return nil, genericclioptions.NoCompatiblePrinterError{OutputFormat: &b.OutputFormat, AllowedFormats: b.AllowedFormats()}
}
