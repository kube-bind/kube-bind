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
	"fmt"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/require"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

func TestFlags(t *testing.T) {
	cmd := cobra.Command{}
	opts := NewBindOptions(genericclioptions.IOStreams{})
	opts.AddCmdFlags(&cmd)

	all := sets.NewString()
	cmd.Flags().VisitAll(func(flag *pflag.Flag) {
		all.Insert(flag.Name)
		if flag.Shorthand != "" {
			all.Insert(flag.Shorthand)
		}
		if flag.ShorthandDeprecated != "" {
			all.Insert(flag.ShorthandDeprecated)
		}
	})

	missing := all.Difference(PassOnFlags).Difference(LocalFlags)
	for _, flag := range missing.List() {
		fmt.Printf("%q,\n", flag)
	}
	require.Empty(t, missing.List())
}
