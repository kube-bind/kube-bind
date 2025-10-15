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

package resources

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	kubebindv1alpha2 "github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2"
)

func TestSelector_IsClaimed(t *testing.T) {
	tests := []struct {
		name     string
		selector kubebindv1alpha2.Selector
		obj      *unstructured.Unstructured
		want     bool
	}{
		{
			name:     "empty selector should select all",
			selector: kubebindv1alpha2.Selector{},
			obj: &unstructured.Unstructured{
				Object: map[string]any{
					"metadata": map[string]any{
						"name":      "test-obj",
						"namespace": "test-ns",
						"labels": map[string]any{
							"app": "test",
						},
					},
				},
			},
			want: true,
		},
		{
			name: "label selector should match labels",
			selector: kubebindv1alpha2.Selector{
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": "test",
					},
				},
			},
			obj: &unstructured.Unstructured{
				Object: map[string]any{
					"metadata": map[string]any{
						"name":      "test-obj",
						"namespace": "test-ns",
						"labels": map[string]any{
							"app": "test",
						},
					},
				},
			},
			want: true,
		},
		{
			name: "label selector should not match different labels",
			selector: kubebindv1alpha2.Selector{
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": "different",
					},
				},
			},
			obj: &unstructured.Unstructured{
				Object: map[string]any{
					"metadata": map[string]any{
						"name":      "test-obj",
						"namespace": "test-ns",
						"labels": map[string]any{
							"app": "test",
						},
					},
				},
			},
			want: false,
		},
		{
			name: "named resource selector should match exact name and namespace",
			selector: kubebindv1alpha2.Selector{
				NamedResources: []kubebindv1alpha2.NamedResource{
					{
						Name:      "test-obj",
						Namespace: "test-ns",
					},
				},
			},
			obj: &unstructured.Unstructured{
				Object: map[string]any{
					"metadata": map[string]any{
						"name":      "test-obj",
						"namespace": "test-ns",
					},
				},
			},
			want: true,
		},
		{
			name: "named resource selector should match name when namespace is empty",
			selector: kubebindv1alpha2.Selector{
				NamedResources: []kubebindv1alpha2.NamedResource{
					{
						Name:      "test-obj",
						Namespace: "",
					},
				},
			},
			obj: &unstructured.Unstructured{
				Object: map[string]any{
					"metadata": map[string]any{
						"name":      "test-obj",
						"namespace": "test-ns",
					},
				},
			},
			want: true,
		},
		{
			name: "named resource selector should not match different name",
			selector: kubebindv1alpha2.Selector{
				NamedResources: []kubebindv1alpha2.NamedResource{
					{
						Name:      "other-obj",
						Namespace: "test-ns",
					},
				},
			},
			obj: &unstructured.Unstructured{
				Object: map[string]any{
					"metadata": map[string]any{
						"name":      "test-obj",
						"namespace": "test-ns",
					},
				},
			},
			want: false,
		},
		{
			name: "named resource selector should not match different namespace",
			selector: kubebindv1alpha2.Selector{
				NamedResources: []kubebindv1alpha2.NamedResource{
					{
						Name:      "test-obj",
						Namespace: "other-ns",
					},
				},
			},
			obj: &unstructured.Unstructured{
				Object: map[string]any{
					"metadata": map[string]any{
						"name":      "test-obj",
						"namespace": "test-ns",
					},
				},
			},
			want: false,
		},
		{
			name: "named resource selector should match one of multiple resources",
			selector: kubebindv1alpha2.Selector{
				NamedResources: []kubebindv1alpha2.NamedResource{
					{
						Name:      "other-obj",
						Namespace: "test-ns",
					},
					{
						Name:      "test-obj",
						Namespace: "test-ns",
					},
				},
			},
			obj: &unstructured.Unstructured{
				Object: map[string]any{
					"metadata": map[string]any{
						"name":      "test-obj",
						"namespace": "test-ns",
					},
				},
			},
			want: true,
		},
		{
			name: "label selector with object having no labels should not match",
			selector: kubebindv1alpha2.Selector{
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": "test",
					},
				},
			},
			obj: &unstructured.Unstructured{
				Object: map[string]any{
					"metadata": map[string]any{
						"name":      "test-obj",
						"namespace": "test-ns",
					},
				},
			},
			want: false,
		},
		{
			name: "combination of label selector and named resource should match when both match",
			selector: kubebindv1alpha2.Selector{
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": "test",
					},
				},
				NamedResources: []kubebindv1alpha2.NamedResource{
					{
						Name:      "test-obj",
						Namespace: "test-ns",
					},
				},
			},
			obj: &unstructured.Unstructured{
				Object: map[string]any{
					"metadata": map[string]any{
						"name":      "test-obj",
						"namespace": "test-ns",
						"labels": map[string]any{
							"app": "test",
						},
					},
				},
			},
			want: true,
		},
		{
			name: "combination of label selector and named resource should not match when label doesn't match",
			selector: kubebindv1alpha2.Selector{
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": "different",
					},
				},
				NamedResources: []kubebindv1alpha2.NamedResource{
					{
						Name:      "test-obj",
						Namespace: "test-ns",
					},
				},
			},
			obj: &unstructured.Unstructured{
				Object: map[string]any{
					"metadata": map[string]any{
						"name":      "test-obj",
						"namespace": "test-ns",
						"labels": map[string]any{
							"app": "test",
						},
					},
				},
			},
			want: false,
		},
		{
			name: "combination of label selector and named resource should not match when name doesn't match",
			selector: kubebindv1alpha2.Selector{
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": "test",
					},
				},
				NamedResources: []kubebindv1alpha2.NamedResource{
					{
						Name:      "other-obj",
						Namespace: "test-ns",
					},
				},
			},
			obj: &unstructured.Unstructured{
				Object: map[string]any{
					"metadata": map[string]any{
						"name":      "test-obj",
						"namespace": "test-ns",
						"labels": map[string]any{
							"app": "test",
						},
					},
				},
			},
			want: false,
		},
		{
			name: "label-only-secret test case - should not sync when has label but not in named resource list",
			selector: kubebindv1alpha2.Selector{
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": "secrets",
					},
				},
				NamedResources: []kubebindv1alpha2.NamedResource{
					{
						Name:      "test-secret",
						Namespace: "default",
					},
					{
						Name:      "named-secret-1",
						Namespace: "default",
					},
					{
						Name:      "named-secret-2",
						Namespace: "default",
					},
				},
			},
			obj: &unstructured.Unstructured{
				Object: map[string]any{
					"metadata": map[string]any{
						"name":      "label-only-secret",
						"namespace": "default",
						"labels": map[string]any{
							"app": "secrets",
						},
					},
				},
			},
			want: false, // Should NOT match because name is not in named resource list
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsClaimed(tt.selector, tt.obj)
			if got != tt.want {
				t.Errorf("IsClaimed() = %v, want %v", got, tt.want)
			}
		})
	}
}
