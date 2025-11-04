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

package framework

import (
	"errors"
	"io"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
)

func ignoreEof(err error) error {
	if errors.Is(err, io.EOF) {
		return nil
	}
	return err
}

// ApplyFiles reads the given file paths and applies the manifests
// contained in them to the cluster specified by config.
// This is equivalent to `kubectl apply -f <file>`.
func ApplyFiles(t testing.TB, config *rest.Config, filePaths ...string) {
	t.Helper()
	t.Logf("Reading manifests from files: %v", filePaths)

	manifests := []any{}
	for _, filePath := range filePaths {
		f, err := os.Open(filePath)
		require.NoError(t, err, "Failed to open file %s", filePath)
		defer f.Close()

		dec := yaml.NewDecoder(f)
		for {
			var node yaml.Node
			err := dec.Decode(&node)
			require.NoError(t, ignoreEof(err), "Failed to decode YAML from file %s", filePath)
			if errors.Is(err, io.EOF) {
				break
			}
			manifests = append(manifests, node)
		}
	}

	ApplyManifest(t, config, manifests...)
}

// ApplyManifest applies the given manifests to the cluster specified by config.
func ApplyManifest(t testing.TB, config *rest.Config, manifests ...any) {
	t.Helper()
	t.Logf("Applying %d manifests", len(manifests))

	dynamicClient := DynamicClient(t, config)
	discoveryClient := DiscoveryClient(t, config)
	mapper := restmapper.NewDeferredDiscoveryRESTMapper(memory.NewMemCacheClient(discoveryClient))

	for _, manifest := range manifests {
		var u unstructured.Unstructured

		switch m := manifest.(type) {
		case unstructured.Unstructured:
			u = m
		case string:
			obj := map[string]any{}
			err := yaml.Unmarshal([]byte(m), &obj)
			require.NoError(t, err, "Failed to unmarshal manifest")
			u.Object = obj
		case []byte:
			obj := map[string]any{}
			err := yaml.Unmarshal(m, &obj)
			require.NoError(t, err, "Failed to unmarshal manifest")
			u.Object = obj
		case yaml.Node:
			obj := map[string]any{}
			err := m.Decode(&obj)
			require.NoError(t, err, "Failed to decode YAML node")
			u.Object = obj
		default:
			require.Fail(t, "Unsupported manifest type %T", m)
		}

		gvk := u.GroupVersionKind()

		t.Logf("Creating resource %s/%s of type %s", u.GetNamespace(), u.GetName(), gvk.String())
		require.Eventually(t, func() bool {
			t.Logf("Finding REST mapping for GVK %#v", gvk)
			mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
			if err != nil {
				t.Logf("Error finding REST mapping for GVK %s: %v", gvk.String(), err)
				return false
			}

			_, err = dynamicClient.Resource(mapping.Resource).
				Namespace(u.GetNamespace()).
				Apply(t.Context(), u.GetName(), &u, metav1.ApplyOptions{FieldManager: "kube-bind-test-framework"})
			if err != nil {
				t.Logf("Error creating resource %s/%s of type %s: %v", u.GetNamespace(), u.GetName(), gvk.String(), err)
				return false
			}
			return true
		}, wait.ForeverTestTimeout, time.Millisecond*100)
	}
}
