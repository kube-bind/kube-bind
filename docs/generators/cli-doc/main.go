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

package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/kcp-dev/kcp/hack/third_party/github.com/spf13/cobra/doc"
	"github.com/spf13/cobra"

	bindcmd "github.com/kube-bind/kube-bind/cli/cmd/kubectl-bind/cmd"
	"github.com/kube-bind/kube-bind/cli/pkg/help"
)

func main() {
	output := flag.String("output", "", "Path to output directory")
	flag.Parse()

	if output == nil || *output == "" {
		fmt.Fprintln(os.Stderr, "output is required")
		os.Exit(1)
	}

	if err := os.MkdirAll(*output, 0755); err != nil {
		log.Fatalf("Failed to re-create docs directory: %v", err)
	}

	rootCmd := bindcmd.KubectlBindCommand()

	// remove the indentation for examples, as they look weird in the rendered markdown
	unindentExamples(rootCmd)

	if err := doc.GenMarkdownTree(rootCmd, *output); err != nil {
		log.Fatalf("Failed to generate docs: %v", err)
	}
}

func unindentExamples(cmd *cobra.Command) {
	cmd.Example = help.RemoveIndentation(cmd.Example)

	for _, child := range cmd.Commands() {
		unindentExamples(child)
	}
}
