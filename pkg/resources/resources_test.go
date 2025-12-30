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
		isolation                      kubebindv1alpha2.Isolation
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
			isolation: kubebindv1alpha2.IsolationPrefixed,
			want:      false,
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
			isolation: kubebindv1alpha2.IsolationPrefixed,
			want:      true,
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
			isolation: kubebindv1alpha2.IsolationPrefixed,
			want:      true,
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
			isolation: kubebindv1alpha2.IsolationPrefixed,
			want:      false,
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
			isolation: kubebindv1alpha2.IsolationPrefixed,
			want:      false,
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
			isolation: kubebindv1alpha2.IsolationPrefixed,
			want:      true,
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
			isolation: kubebindv1alpha2.IsolationPrefixed,
			want:      false,
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
			isolation: kubebindv1alpha2.IsolationPrefixed,
			want:      true,
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
			isolation: kubebindv1alpha2.IsolationPrefixed,
			want:      true,
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
			isolation: kubebindv1alpha2.IsolationPrefixed,
			want:      true,
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
			isolation: kubebindv1alpha2.IsolationPrefixed,
			want:      true, // Should match because label selector matches (OR logic)
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
			isolation: kubebindv1alpha2.IsolationPrefixed,
			want:      true,
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
			isolation: kubebindv1alpha2.IsolationPrefixed,
			want:      true,
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
			isolation: kubebindv1alpha2.IsolationPrefixed,
			want:      false,
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
			isolation: kubebindv1alpha2.IsolationPrefixed,
			want:      false,
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
			isolation: kubebindv1alpha2.IsolationPrefixed,
			want:      true,
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
			isolation: kubebindv1alpha2.IsolationPrefixed,
			want:      true,
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
			isolation: kubebindv1alpha2.IsolationPrefixed,
			want:      false, // Should not match because [*] syntax is not supported by gjson
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
			isolation: kubebindv1alpha2.IsolationPrefixed,
			want:      false,
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
			isolation: kubebindv1alpha2.IsolationPrefixed,
			want:      false,
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
			isolation: kubebindv1alpha2.IsolationPrefixed,
			want:      false, // Should NOT match because it doesn't have app=sheriff label and is not referenced by Sheriff
		},
		{
			name: "cert-manager secret reference - should match when certificate references secret via secretName",
			potentiallyReferencedResources: &unstructured.UnstructuredList{
				Items: []unstructured.Unstructured{
					{
						Object: map[string]any{
							"apiVersion": "cert-manager.io/v1",
							"kind":       "Certificate",
							"metadata": map[string]any{
								"name":      "my-tls-cert",
								"namespace": "default",
							},
							"spec": map[string]any{
								"secretName": "my-tls-cert",
								"commonName": "my-ca",
								"isCA":       true,
								"issuerRef": map[string]any{
									"kind": "ClusterIssuer",
									"name": "kcp-ca",
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
							Group:    "cert-manager.io",
							Resource: "certificates",
						},
						JSONPath: &kubebindv1alpha2.JSONPath{
							Name: "spec.secretName",
						},
					},
				},
			},
			obj: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "v1",
					"kind":       "Secret",
					"metadata": map[string]any{
						"name":      "my-tls-cert",
						"namespace": "default",
						"annotations": map[string]any{
							"cert-manager.io/certificate-name": "my-tls-cert",
						},
					},
					"type": "kubernetes.io/tls",
				},
			},
			isolation: kubebindv1alpha2.IsolationPrefixed,
			want:      true, // Should match because certificate's .spec.secretName equals secret name and no namespace JSONPath means namespace matching is handled by caller
		},
		{
			name: "cert-manager secret reference - should not match when certificate references different secret",
			potentiallyReferencedResources: &unstructured.UnstructuredList{
				Items: []unstructured.Unstructured{
					{
						Object: map[string]any{
							"apiVersion": "cert-manager.io/v1",
							"kind":       "Certificate",
							"metadata": map[string]any{
								"name":      "my-tls-cert",
								"namespace": "default",
							},
							"spec": map[string]any{
								"secretName": "other-secret",
								"commonName": "my-ca",
								"isCA":       true,
							},
						},
					},
				},
			},
			selector: kubebindv1alpha2.Selector{
				References: []kubebindv1alpha2.SelectorReference{
					{
						GroupResource: kubebindv1alpha2.GroupResource{
							Group:    "cert-manager.io",
							Resource: "certificates",
						},
						JSONPath: &kubebindv1alpha2.JSONPath{
							Name: "spec.secretName",
						},
					},
				},
			},
			obj: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "v1",
					"kind":       "Secret",
					"metadata": map[string]any{
						"name":      "my-tls-cert",
						"namespace": "default",
					},
					"type": "kubernetes.io/tls",
				},
			},
			isolation: kubebindv1alpha2.IsolationPrefixed,
			want:      false, // Should not match because certificate's .spec.secretName is "other-secret", not "my-tls-cert"
		},
		{
			name: "cert-manager secret reference - cross-namespace scenario with no namespace JSONPath",
			potentiallyReferencedResources: &unstructured.UnstructuredList{
				Items: []unstructured.Unstructured{
					{
						Object: map[string]any{
							"apiVersion": "cert-manager.io/v1",
							"kind":       "Certificate",
							"metadata": map[string]any{
								"name":      "my-tls-cert",
								"namespace": "default",
							},
							"spec": map[string]any{
								"secretName": "my-tls-cert",
								"commonName": "my-ca",
								"isCA":       true,
							},
						},
					},
				},
			},
			selector: kubebindv1alpha2.Selector{
				References: []kubebindv1alpha2.SelectorReference{
					{
						GroupResource: kubebindv1alpha2.GroupResource{
							Group:    "cert-manager.io",
							Resource: "certificates",
						},
						JSONPath: &kubebindv1alpha2.JSONPath{
							Name: "spec.secretName",
						},
					},
				},
			},
			obj: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "v1",
					"kind":       "Secret",
					"metadata": map[string]any{
						"name":      "my-tls-cert",
						"namespace": "remapped-namespace", // Different namespace simulating provider-side remapping
					},
					"type": "kubernetes.io/tls",
				},
			},
			isolation: kubebindv1alpha2.IsolationPrefixed,
			want:      true, // Should match because when no namespace JSONPath is provided, namespace matching is delegated to caller
		},
		{
			name: "namespace inherited from referencing object when JSONPath not provided in the namespaced isolation mode",
			potentiallyReferencedResources: &unstructured.UnstructuredList{
				Items: []unstructured.Unstructured{
					{
						Object: map[string]any{
							"metadata": map[string]any{
								"name":      "mangodb-instance",
								"namespace": "default",
							},
							"spec": map[string]any{
								"secret": map[string]any{
									"name": "my-secret",
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
							Group:    "mangodb.com",
							Resource: "mangodbs",
						},
						JSONPath: &kubebindv1alpha2.JSONPath{
							Name: "spec.secret.name",
						},
					},
				},
			},
			obj: &unstructured.Unstructured{
				Object: map[string]any{
					"metadata": map[string]any{
						"name":      "my-secret",
						"namespace": "default",
					},
				},
			},
			isolation: kubebindv1alpha2.IsolationNamespaced,
			want:      true,
		},
		{
			name: "namespace inheritance fails when in different namespace in the namespaced isolation mode",
			potentiallyReferencedResources: &unstructured.UnstructuredList{
				Items: []unstructured.Unstructured{
					{
						Object: map[string]any{
							"metadata": map[string]any{
								"name":      "mangodb-instance",
								"namespace": "default",
							},
							"spec": map[string]any{
								"secret": map[string]any{
									"name": "my-secret",
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
							Group:    "mangodb.com",
							Resource: "mangodbs",
						},
						JSONPath: &kubebindv1alpha2.JSONPath{
							Name: "spec.secret.name",
						},
					},
				},
			},
			obj: &unstructured.Unstructured{
				Object: map[string]any{
					"metadata": map[string]any{
						"name":      "my-secret",
						"namespace": "other-namespace",
					},
				},
			},
			isolation: kubebindv1alpha2.IsolationNamespaced,
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := klog.Background().V(4)
			got := IsClaimed(logger, tt.selector, tt.obj, tt.potentiallyReferencedResources, tt.isolation)
			if got != tt.want {
				t.Errorf("IsClaimed() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestReferenceSelector_IsolationMode(t *testing.T) {
	tests := []struct {
		name              string
		isolation         kubebindv1alpha2.Isolation
		exportedResources []kubebindv1alpha2.APIServiceExportResource
		reference         kubebindv1alpha2.SelectorReference
		shouldAllow       bool
	}{
		{
			name:      "IsolationNone allows all references",
			isolation: kubebindv1alpha2.IsolationNone,
			exportedResources: []kubebindv1alpha2.APIServiceExportResource{
				{GroupResource: kubebindv1alpha2.GroupResource{Group: "mangodb.com", Resource: "mangodbs"}},
			},
			reference: kubebindv1alpha2.SelectorReference{
				GroupResource: kubebindv1alpha2.GroupResource{Group: "secret.com", Resource: "secrets"},
			},
			shouldAllow: true,
		},
		{
			name:      "IsolationNamespaced blocks non-exported references",
			isolation: kubebindv1alpha2.IsolationNamespaced,
			exportedResources: []kubebindv1alpha2.APIServiceExportResource{
				{GroupResource: kubebindv1alpha2.GroupResource{Group: "mangodb.com", Resource: "mangodbs"}},
			},
			reference: kubebindv1alpha2.SelectorReference{
				GroupResource: kubebindv1alpha2.GroupResource{Group: "secret.com", Resource: "secrets"},
			},
			shouldAllow: false,
		},
		{
			name:      "IsolationPrefixed blocks non-exported references",
			isolation: kubebindv1alpha2.IsolationPrefixed,
			exportedResources: []kubebindv1alpha2.APIServiceExportResource{
				{GroupResource: kubebindv1alpha2.GroupResource{Group: "mangodb.com", Resource: "mangodbs"}},
			},
			reference: kubebindv1alpha2.SelectorReference{
				GroupResource: kubebindv1alpha2.GroupResource{Group: "secret.com", Resource: "secrets"},
			},
			shouldAllow: false,
		},
		{
			name:      "IsolationPrefixed allows exported references",
			isolation: kubebindv1alpha2.IsolationPrefixed,
			exportedResources: []kubebindv1alpha2.APIServiceExportResource{
				{GroupResource: kubebindv1alpha2.GroupResource{Group: "mangodb.com", Resource: "mangodbs"}},
			},
			reference: kubebindv1alpha2.SelectorReference{
				GroupResource: kubebindv1alpha2.GroupResource{Group: "mangodb.com", Resource: "mangodbs"},
			},
			shouldAllow: true,
		},
		{
			name:      "IsolationNamespaced allows exported references",
			isolation: kubebindv1alpha2.IsolationNamespaced,
			exportedResources: []kubebindv1alpha2.APIServiceExportResource{
				{GroupResource: kubebindv1alpha2.GroupResource{Group: "mangodb.com", Resource: "mangodbs"}},
			},
			reference: kubebindv1alpha2.SelectorReference{
				GroupResource: kubebindv1alpha2.GroupResource{Group: "mangodb.com", Resource: "mangodbs"},
			},
			shouldAllow: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			export := &kubebindv1alpha2.APIServiceExport{
				Spec: kubebindv1alpha2.APIServiceExportSpec{
					ClusterScopedIsolation: tt.isolation,
					Resources:              tt.exportedResources,
				},
			}
			got := isReferenceAllowed(tt.reference, export)
			if got != tt.shouldAllow {
				t.Errorf("isReferenceAllowed() = %v, want %v", got, tt.shouldAllow)
			}
		})
	}
}
