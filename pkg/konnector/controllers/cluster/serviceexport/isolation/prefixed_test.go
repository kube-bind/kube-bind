/*
Copyright 2023 The Kube Bind Authors.

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

package isolation

import (
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/types"
)

func TestPrefixedToProviderKey(t *testing.T) {
	tests := []struct {
		name       string
		providerNs string
		objectName string
		expected   string
	}{
		{
			name:       "basic testcase",
			providerNs: "kube-bind-zlp9m",
			objectName: "example-foo",
			expected:   "kube-bind-zlp9m-example-foo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			strategy := NewPrefixed(tt.providerNs, "irrelevant-uid")

			result, err := strategy.ToProviderKey(types.NamespacedName{Name: tt.objectName})
			require.NoError(t, err)
			require.NotNil(t, result)
			require.Equal(t, "", result.Namespace)
			require.Equal(t, tt.expected, result.Name)
		})
	}
}

func TestPrefixedToConsumerKey(t *testing.T) {
	tests := []struct {
		name       string
		providerNs string
		objectName string
		expected   string
	}{
		{
			name:       "basic testcase",
			providerNs: "kube-bind-zlp9m",
			objectName: "kube-bind-zlp9m-example-foo",
			expected:   "example-foo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			strategy := NewPrefixed(tt.providerNs, "irrelevant-uid")

			result, err := strategy.ToConsumerKey(types.NamespacedName{Name: tt.objectName})
			require.NoError(t, err)
			require.NotNil(t, result)
			require.Equal(t, "", result.Namespace)
			require.Equal(t, tt.expected, result.Name)
		})
	}
}
