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

package clusterscoped

import (
	"testing"

	"github.com/stretchr/testify/require"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestSwitchToUpstreamName(t *testing.T) {
	tests := []struct {
		name      string
		clusterNs string
		down      string
		expected  string
	}{
		{
			name:      "2up",
			clusterNs: "kube-bind-zlp9m",
			down:      "example-foo",
			expected:  "kube-bind-zlp9m-example-foo",
		},
	}
	for _, tt := range tests {
		obj := unstructured.Unstructured{}
		obj.SetName(tt.down)
		SwitchToUpstreamName(&obj, tt.clusterNs)
		t.Run(tt.name, func(t *testing.T) {
			if actual := obj.GetName(); actual != tt.expected {
				t.Error("SwitchToUpstreamName() error", "expected", tt.expected, "actual", actual)
			}
		})
	}
}

func TestSwitchToDownstreamName(t *testing.T) {
	tests := []struct {
		name      string
		clusterNs string
		up        string
		expected  string
	}{
		{
			name:      "2down",
			clusterNs: "kube-bind-zlp9m",
			up:        "kube-bind-zlp9m-example-foo",
			expected:  "example-foo",
		},
	}
	for _, tt := range tests {
		obj := unstructured.Unstructured{}
		obj.SetName(tt.up)
		SwitchToDownstreamName(&obj, tt.clusterNs)
		t.Run(tt.name, func(t *testing.T) {
			if actual := obj.GetName(); actual != tt.expected {
				t.Error("SwitchToUpstreamName() error", "expected", tt.expected, "actual", actual)
			}
		})
	}
}

func TestInjectClusterNs(t *testing.T) {
	tests := []struct {
		name         string
		obj          unstructured.Unstructured
		clusterNs    string
		clusterNsUID string
		expected     string
		wantErr      bool
	}{
		{
			name:         "noExistingClusterNs",
			obj:          unstructured.Unstructured{},
			clusterNs:    "kube-bind-zlp9m",
			clusterNsUID: "real-identity",
			expected:     "kube-bind-zlp9m",
			wantErr:      false,
		},
		{
			name:         "oneExistingClusterNs",
			obj:          newObjectWithClusterNs("kube-bind-zlp9m"),
			clusterNs:    "kube-bind-s85lc",
			clusterNsUID: "real-indentity",
			expected:     "kube-bind-zlp9m",
			wantErr:      true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := InjectClusterNs(&tt.obj, tt.clusterNs, tt.clusterNsUID); (err != nil) != tt.wantErr {
				actual := tt.obj.GetAnnotations()[ClusterNsAnnotationKey]
				t.Error("InjectClusterNs() error", "error", err, "expected", tt.expected, "actual", actual)
			} else if err == nil {
				actual := tt.obj.GetAnnotations()[ClusterNsAnnotationKey]
				require.Equal(t, tt.expected, actual)
			} else {
				t.Log("expected error", err)
			}
		})
	}
}

func TestExtractClusterNs(t *testing.T) {
	tests := []struct {
		name     string
		obj      unstructured.Unstructured
		expected string
		wantErr  bool
	}{
		{
			name:     "oneExistingClusterNs",
			obj:      newObjectWithClusterNs("kube-bind-zlp9m"),
			expected: "kube-bind-zlp9m",
			wantErr:  false,
		},
		{
			name:     "noExistingClusterNs",
			obj:      unstructured.Unstructured{},
			expected: "",
			wantErr:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if actual, err := ExtractClusterNs(&tt.obj); (err != nil) != tt.wantErr {
				t.Error("ExtractClusterNs() error", "error", err, "expected", tt.expected, "actual", actual)
			} else if err == nil {
				require.Equal(t, tt.expected, actual)
			} else {
				t.Log("expected error", err)
			}
		})
	}
}

func TestClearClusterNs(t *testing.T) {
	tests := []struct {
		name      string
		obj       unstructured.Unstructured
		clusterNs string
		expected  int
		wantErr   bool
	}{
		{
			name:      "oneExistingClusterNs",
			obj:       newObjectWithClusterNs("kube-bind-zlp9m"),
			clusterNs: "kube-bind-zlp9m",
			expected:  0,
			wantErr:   false,
		},
		{
			name:      "noExistingClusterNs",
			obj:       unstructured.Unstructured{},
			clusterNs: "not-applicable",
			expected:  0,
			wantErr:   false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ClearClusterNs(&tt.obj, tt.clusterNs); (err != nil) != tt.wantErr {
				actual := len(tt.obj.GetOwnerReferences())
				t.Error("ClearClusterNs() error", "error", err, "expected", tt.expected, "actual", actual)
			} else if err == nil {
				actual := len(tt.obj.GetOwnerReferences())
				require.Equal(t, tt.expected, actual)
			} else {
				t.Log("expected error", err)
			}
		})
	}
}

func newObjectWithClusterNs(name string) unstructured.Unstructured {
	obj := unstructured.Unstructured{}
	ans := map[string]string{
		ClusterNsAnnotationKey: name,
	}
	obj.SetAnnotations(ans)
	ors := []metav1.OwnerReference{{
		APIVersion: "v1",
		Kind:       "Namespace",
		Name:       name,
	}}
	obj.SetOwnerReferences(ors)
	return obj
}
