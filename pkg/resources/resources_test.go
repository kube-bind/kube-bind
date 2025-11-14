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
	"k8s.io/klog/v2"

	kubebindv1alpha2 "github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2"
)

func TestSelector_IsClaimed(t *testing.T) {
	tests := []struct {
		name                           string
		selector                       kubebindv1alpha2.Selector
		obj                            *unstructured.Unstructured
		potentiallyReferencedResources *unstructured.UnstructuredList
		want                           bool
	}{
		{
			name:                           "empty selector should select nothing",
			selector:                       kubebindv1alpha2.Selector{},
			potentiallyReferencedResources: nil,
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
			name:                           "label selector should match labels",
			potentiallyReferencedResources: nil,
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
			name:                           "label selector should not match different labels",
			potentiallyReferencedResources: nil,
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
			name:                           "named resource selector should match exact name and namespace",
			potentiallyReferencedResources: nil,
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
			name:                           "named resource selector should match name when namespace is empty",
			potentiallyReferencedResources: nil,
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
			name:                           "named resource selector should not match different name",
			potentiallyReferencedResources: nil,
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
			name:                           "named resource selector should not match different namespace",
			potentiallyReferencedResources: nil,
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
			name:                           "named resource selector should match one of multiple resources",
			potentiallyReferencedResources: nil,
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
			name:                           "label selector with object having no labels should not match",
			potentiallyReferencedResources: nil,
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
			name:                           "combination of label selector and named resource should match when both match",
			potentiallyReferencedResources: nil,
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
			name:                           "combination of label selector and named resource should match when named resource matches (OR logic)",
			potentiallyReferencedResources: nil,
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
			want: true,
		},
		{
			name:                           "combination of label selector and named resource should match when label matches (OR logic)",
			potentiallyReferencedResources: nil,
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
			want: true,
		},
		{
			name:                           "label-only-secret test case - should sync when has matching label (OR logic)",
			potentiallyReferencedResources: nil,
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
			want: true, // Should match because label selector matches (OR logic)
		},
		// JSONPath Reference tests
		{
			name: "reference selector should match when JSONPath extracts matching name",
			potentiallyReferencedResources: &unstructured.UnstructuredList{
				Items: []unstructured.Unstructured{
					{
						Object: map[string]any{
							"metadata": map[string]any{
								"name":      "config-obj",
								"namespace": "default",
							},
							"spec": map[string]any{
								"secretRef": map[string]any{
									"name": "test-secret",
								},
							},
						},
					},
				},
			},
			selector: kubebindv1alpha2.Selector{
				References: []kubebindv1alpha2.SelectorReference{
					{
						GroupResource: kubebindv1alpha2.GroupResource{
							Group:    "apps",
							Resource: "deployments",
						},
						JSONPath: &kubebindv1alpha2.JSONPath{
							Name: "spec.secretRef.name",
						},
					},
				},
			},
			obj: &unstructured.Unstructured{
				Object: map[string]any{
					"metadata": map[string]any{
						"name":      "test-secret",
						"namespace": "default",
					},
				},
			},
			want: true,
		},
		{
			name: "reference selector should match when JSONPath extracts matching name and namespace",
			potentiallyReferencedResources: &unstructured.UnstructuredList{
				Items: []unstructured.Unstructured{
					{
						Object: map[string]any{
							"metadata": map[string]any{
								"name":      "config-obj",
								"namespace": "default",
							},
							"spec": map[string]any{
								"secretRef": map[string]any{
									"name":      "test-secret",
									"namespace": "test-namespace",
								},
							},
						},
					},
				},
			},
			selector: kubebindv1alpha2.Selector{
				References: []kubebindv1alpha2.SelectorReference{
					{
						GroupResource: kubebindv1alpha2.GroupResource{
							Group:    "apps",
							Resource: "deployments",
						},
						JSONPath: &kubebindv1alpha2.JSONPath{
							Name:      "spec.secretRef.name",
							Namespace: "spec.secretRef.namespace",
						},
					},
				},
			},
			obj: &unstructured.Unstructured{
				Object: map[string]any{
					"metadata": map[string]any{
						"name":      "test-secret",
						"namespace": "test-namespace",
					},
				},
			},
			want: true,
		},
		{
			name: "reference selector should not match when JSONPath extracts different name",
			potentiallyReferencedResources: &unstructured.UnstructuredList{
				Items: []unstructured.Unstructured{
					{
						Object: map[string]any{
							"metadata": map[string]any{
								"name":      "config-obj",
								"namespace": "default",
							},
							"spec": map[string]any{
								"secretRef": map[string]any{
									"name": "other-secret",
								},
							},
						},
					},
				},
			},
			selector: kubebindv1alpha2.Selector{
				References: []kubebindv1alpha2.SelectorReference{
					{
						GroupResource: kubebindv1alpha2.GroupResource{
							Group:    "apps",
							Resource: "deployments",
						},
						JSONPath: &kubebindv1alpha2.JSONPath{
							Name: "spec.secretRef.name",
						},
					},
				},
			},
			obj: &unstructured.Unstructured{
				Object: map[string]any{
					"metadata": map[string]any{
						"name":      "test-secret",
						"namespace": "default",
					},
				},
			},
			want: false,
		},
		{
			name: "reference selector should not match when JSONPath extracts different namespace",
			potentiallyReferencedResources: &unstructured.UnstructuredList{
				Items: []unstructured.Unstructured{
					{
						Object: map[string]any{
							"metadata": map[string]any{
								"name":      "config-obj",
								"namespace": "default",
							},
							"spec": map[string]any{
								"secretRef": map[string]any{
									"name":      "test-secret",
									"namespace": "other-namespace",
								},
							},
						},
					},
				},
			},
			selector: kubebindv1alpha2.Selector{
				References: []kubebindv1alpha2.SelectorReference{
					{
						GroupResource: kubebindv1alpha2.GroupResource{
							Group:    "apps",
							Resource: "deployments",
						},
						JSONPath: &kubebindv1alpha2.JSONPath{
							Name:      "spec.secretRef.name",
							Namespace: "spec.secretRef.namespace",
						},
					},
				},
			},
			obj: &unstructured.Unstructured{
				Object: map[string]any{
					"metadata": map[string]any{
						"name":      "test-secret",
						"namespace": "test-namespace",
					},
				},
			},
			want: false,
		},
		{
			name: "reference selector should handle array JSONPath results",
			potentiallyReferencedResources: &unstructured.UnstructuredList{
				Items: []unstructured.Unstructured{
					{
						Object: map[string]any{
							"metadata": map[string]any{
								"name":      "config-obj",
								"namespace": "default",
							},
							"spec": map[string]any{
								"secretRefs": []any{
									map[string]any{"name": "secret1"},
									map[string]any{"name": "test-secret"},
									map[string]any{"name": "secret3"},
								},
							},
						},
					},
				},
			},
			selector: kubebindv1alpha2.Selector{
				References: []kubebindv1alpha2.SelectorReference{
					{
						GroupResource: kubebindv1alpha2.GroupResource{
							Group:    "apps",
							Resource: "deployments",
						},
						JSONPath: &kubebindv1alpha2.JSONPath{
							Name: "spec.secretRefs.#.name",
						},
					},
				},
			},
			obj: &unstructured.Unstructured{
				Object: map[string]any{
					"metadata": map[string]any{
						"name":      "test-secret",
						"namespace": "default",
					},
				},
			},
			want: true,
		},
		{
			name: "reference selector should handle array JSONPath with namespace extraction",
			potentiallyReferencedResources: &unstructured.UnstructuredList{
				Items: []unstructured.Unstructured{
					{
						Object: map[string]any{
							"metadata": map[string]any{
								"name":      "config-obj",
								"namespace": "default",
							},
							"spec": map[string]any{
								"secretRefs": []any{
									map[string]any{
										"name":      "secret1",
										"namespace": "ns1",
									},
									map[string]any{
										"name":      "test-secret",
										"namespace": "test-namespace",
									},
									map[string]any{
										"name":      "secret3",
										"namespace": "ns3",
									},
								},
							},
						},
					},
				},
			},
			selector: kubebindv1alpha2.Selector{
				References: []kubebindv1alpha2.SelectorReference{
					{
						GroupResource: kubebindv1alpha2.GroupResource{
							Group:    "apps",
							Resource: "deployments",
						},
						JSONPath: &kubebindv1alpha2.JSONPath{
							Name:      "spec.secretRefs.#.name",
							Namespace: "spec.secretRefs.#.namespace",
						},
					},
				},
			},
			obj: &unstructured.Unstructured{
				Object: map[string]any{
					"metadata": map[string]any{
						"name":      "test-secret",
						"namespace": "test-namespace",
					},
				},
			},
			want: true,
		},
		{
			name: "reference selector should not match with bracket wildcard syntax (unsupported)",
			potentiallyReferencedResources: &unstructured.UnstructuredList{
				Items: []unstructured.Unstructured{
					{
						Object: map[string]any{
							"metadata": map[string]any{
								"name":      "config-obj",
								"namespace": "default",
							},
							"spec": map[string]any{
								"secretRefs": []any{
									map[string]any{
										"name":      "test-secret",
										"namespace": "test-namespace",
									},
								},
							},
						},
					},
				},
			},
			selector: kubebindv1alpha2.Selector{
				References: []kubebindv1alpha2.SelectorReference{
					{
						GroupResource: kubebindv1alpha2.GroupResource{
							Group:    "apps",
							Resource: "deployments",
						},
						JSONPath: &kubebindv1alpha2.JSONPath{
							Name:      "spec.secretRefs[*].name",
							Namespace: "spec.secretRefs[*].namespace",
						},
					},
				},
			},
			obj: &unstructured.Unstructured{
				Object: map[string]any{
					"metadata": map[string]any{
						"name":      "test-secret",
						"namespace": "test-namespace",
					},
				},
			},
			want: false, // Should not match because [*] syntax is not supported by gjson
		},
		{
			name:                           "reference selector should not match when no referenced resources provided",
			potentiallyReferencedResources: nil,
			selector: kubebindv1alpha2.Selector{
				References: []kubebindv1alpha2.SelectorReference{
					{
						GroupResource: kubebindv1alpha2.GroupResource{
							Group:    "apps",
							Resource: "deployments",
						},
						JSONPath: &kubebindv1alpha2.JSONPath{
							Name: "spec.secretRef.name",
						},
					},
				},
			},
			obj: &unstructured.Unstructured{
				Object: map[string]any{
					"metadata": map[string]any{
						"name":      "test-secret",
						"namespace": "default",
					},
				},
			},
			want: false,
		},
		{
			name: "reference selector should not match when JSONPath does not exist",
			potentiallyReferencedResources: &unstructured.UnstructuredList{
				Items: []unstructured.Unstructured{
					{
						Object: map[string]any{
							"metadata": map[string]any{
								"name":      "config-obj",
								"namespace": "default",
							},
							"spec": map[string]any{
								"otherField": "value",
							},
						},
					},
				},
			},
			selector: kubebindv1alpha2.Selector{
				References: []kubebindv1alpha2.SelectorReference{
					{
						GroupResource: kubebindv1alpha2.GroupResource{
							Group:    "apps",
							Resource: "deployments",
						},
						JSONPath: &kubebindv1alpha2.JSONPath{
							Name: "spec.secretRef.name",
						},
					},
				},
			},
			obj: &unstructured.Unstructured{
				Object: map[string]any{
					"metadata": map[string]any{
						"name":      "test-secret",
						"namespace": "default",
					},
				},
			},
			want: false,
		},
		{
			name: "kubeconfig secret case - should not match when only has kube-bind.io/owner label but selector requires app=sheriff",
			potentiallyReferencedResources: &unstructured.UnstructuredList{
				Items: []unstructured.Unstructured{
					{
						Object: map[string]any{
							"apiVersion": "wildwest.dev/v1alpha1",
							"kind":       "Sheriff",
							"metadata": map[string]any{
								"name": "wyatt-earp",
							},
							"spec": map[string]any{
								"secretRefs": []any{
									map[string]any{
										"name":      "sheriff-badge-credentials",
										"namespace": "wild-west",
									},
								},
							},
						},
					},
				},
			},
			selector: kubebindv1alpha2.Selector{
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": "sheriff",
					},
				},
				References: []kubebindv1alpha2.SelectorReference{
					{
						GroupResource: kubebindv1alpha2.GroupResource{
							Group:    "wildwest.dev",
							Resource: "sheriffs",
						},
						JSONPath: &kubebindv1alpha2.JSONPath{
							Name:      "spec.secretRefs.#.name",
							Namespace: "spec.secretRefs.#.namespace",
						},
					},
				},
			},
			obj: &unstructured.Unstructured{
				Object: map[string]any{
					"metadata": map[string]any{
						"name":      "kubeconfig-9d76b",
						"namespace": "kube-bind",
						"labels": map[string]any{
							"kube-bind.io/owner": "consumer",
						},
					},
				},
			},
			want: false, // Should NOT match because it doesn't have app=sheriff label and is not referenced by Sheriff
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := klog.Background().V(4)
			got := IsClaimed(logger, tt.selector, tt.obj, tt.potentiallyReferencedResources)
			if got != tt.want {
				t.Errorf("IsClaimed() = %v, want %v", got, tt.want)
			}
		})
	}
}
