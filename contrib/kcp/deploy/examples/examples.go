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

package examples

import (
	"context"
	"embed"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"

	"github.com/kube-bind/kube-bind/pkg/bootstrap"
)

//go:embed *.yaml
var ExampleBackendAssets embed.FS

func BootstrapExampleBackend(t *testing.T, discoveryClient discovery.DiscoveryInterface, dynamicClient dynamic.Interface, batteriesIncluded sets.Set[string]) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	err := bootstrap.Bootstrap(ctx, discoveryClient, dynamicClient, batteriesIncluded, ExampleBackendAssets)
	require.NoError(t, err)
}
