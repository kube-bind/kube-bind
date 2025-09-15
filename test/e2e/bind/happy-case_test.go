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

package bind

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gopkg.in/headzoo/surf.v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/yaml"

	clusterscoped "github.com/kube-bind/kube-bind/pkg/konnector/controllers/cluster/serviceexport/cluster-scoped"
	kubebindv1alpha2 "github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2"
	providerfixtures "github.com/kube-bind/kube-bind/test/e2e/bind/fixtures/provider"
	"github.com/kube-bind/kube-bind/test/e2e/framework"
)

func TestClusterScoped(t *testing.T) {
	t.Parallel()
	testHappyCase(t, apiextensionsv1.ClusterScoped, kubebindv1alpha2.ClusterScope, false)
	testHappyCase(t, apiextensionsv1.ClusterScoped, kubebindv1alpha2.ClusterScope, true)
}

func TestNamespacedScoped(t *testing.T) {
	t.Parallel()

	testHappyCase(t, apiextensionsv1.NamespaceScoped, kubebindv1alpha2.NamespacedScope, false)
	testHappyCase(t, apiextensionsv1.NamespaceScoped, kubebindv1alpha2.NamespacedScope, true)
	testHappyCase(t, apiextensionsv1.NamespaceScoped, kubebindv1alpha2.ClusterScope, false)
	testHappyCase(t, apiextensionsv1.NamespaceScoped, kubebindv1alpha2.ClusterScope, true)
}

func testHappyCase(
	t *testing.T,
	resourceScope apiextensionsv1.ResourceScope,
	informerScope kubebindv1alpha2.InformerScope,
	withPermissionClaims bool,
) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	t.Logf("Creating provider workspace")
	providerConfig, providerKubeconfig := framework.NewWorkspace(t, framework.ClientConfig(t), framework.WithGenerateName("test-happy-case-provider"))

	t.Logf("Starting backend with random port")
	addr, _ := framework.StartBackend(t, providerConfig, "--kubeconfig="+providerKubeconfig, "--listen-address=:0", "--consumer-scope="+string(informerScope))

	t.Logf("Creating CRD on provider side")
	providerfixtures.Bootstrap(t, framework.DiscoveryClient(t, providerConfig), framework.DynamicClient(t, providerConfig), nil)

	t.Logf("Creating consumer workspace and starting konnector")
	consumerConfig, consumerKubeconfig := framework.NewWorkspace(t, framework.ClientConfig(t), framework.WithGenerateName("test-happy-case-consumer"))
	framework.StartKonnector(t, consumerConfig, "--kubeconfig="+consumerKubeconfig)

	serviceGVR := schema.GroupVersionResource{Group: "mangodb.com", Version: "v1alpha1", Resource: "mangodbs"}
	if resourceScope == apiextensionsv1.ClusterScoped {
		serviceGVR = schema.GroupVersionResource{Group: "bar.io", Version: "v1alpha1", Resource: "foos"}
	}

	consumerClient := framework.DynamicClient(t, consumerConfig).Resource(serviceGVR)
	providerClient := framework.DynamicClient(t, providerConfig).Resource(serviceGVR)

	consumerCoreClient := framework.KubeClient(t, consumerConfig).CoreV1()
	providerCoreClient := framework.KubeClient(t, providerConfig).CoreV1()

	mangodbInstance := `
apiVersion: mangodb.com/v1alpha1
kind: MangoDB
metadata:
  name: test
spec:
  tokenSecret: credentials
`
	fooInstance := `
apiVersion: bar.io/v1alpha1
kind: Foo
metadata:
  name: test
spec:
  deploymentName: test-foo
  replicas: 2
`
	downstreamNs, upstreamNS := "default", "unknown"
	clusterNs, clusterScopedUpInsName := "unknown", "unknown"

	for _, tc := range []struct {
		name string
		step func(t *testing.T)
	}{
		{
			name: "Service is bound dry run",
			step: func(t *testing.T) {
				iostreams, _, bufOut, _ := genericclioptions.NewTestIOStreams()
				authURLDryRunCh := make(chan string, 1)
				go simulateBrowser(t, authURLDryRunCh, serviceGVR.Resource)
				framework.Bind(t, iostreams, authURLDryRunCh, nil, fmt.Sprintf("http://%s/exports", addr.String()), "--kubeconfig", consumerKubeconfig, "--skip-konnector", "--dry-run")
				_, err := yaml.YAMLToJSON(bufOut.Bytes())
				require.NoError(t, err)
			},
		},
		{
			name: "Service is bound",
			step: func(t *testing.T) {
				iostreams, _, _, _ := genericclioptions.NewTestIOStreams()
				authURLCh := make(chan string, 1)
				go simulateBrowser(t, authURLCh, serviceGVR.Resource)
				invocations := make(chan framework.SubCommandInvocation, 1)
				framework.Bind(t, iostreams, authURLCh, invocations, fmt.Sprintf("http://%s/exports", addr.String()), "--kubeconfig", consumerKubeconfig, "--skip-konnector")
				inv := <-invocations
				requireEqualSlicePattern(t, []string{"apiservice", "--remote-kubeconfig-namespace", "*", "--remote-kubeconfig-name", "*", "-f", "-", "--kubeconfig=" + consumerKubeconfig, "--skip-konnector=true", "--no-banner"}, inv.Args)

				// If we are in permissions claims mode - add configmaps & secrets
				if withPermissionClaims {
					var request kubebindv1alpha2.APIServiceExportRequest
					err := json.Unmarshal(inv.Stdin, &request)
					require.NoError(t, err)
					request.Spec.PermissionClaims = []kubebindv1alpha2.PermissionClaim{
						{
							GroupResource: kubebindv1alpha2.GroupResource{
								Group:    "",
								Resource: "configmaps",
							},
							Selector: kubebindv1alpha2.Selector{
								LabelSelector: &metav1.LabelSelector{
									MatchLabels: map[string]string{
										"app": "configmaps",
									},
								},
							},
						},
						{
							GroupResource: kubebindv1alpha2.GroupResource{
								Group:    "",
								Resource: "secrets",
							},
							Selector: kubebindv1alpha2.Selector{
								LabelSelector: &metav1.LabelSelector{
									MatchLabels: map[string]string{
										"app": "secrets",
									},
								},
							},
						},
					}
					payload, err := json.Marshal(request)
					require.NoError(t, err)
					inv.Stdin = payload
				}

				framework.BindAPIService(t, inv.Stdin, "", inv.Args...)

				t.Logf("Waiting for %s CRD to be created on consumer side", serviceGVR.Resource)
				crdClient := framework.ApiextensionsClient(t, consumerConfig).ApiextensionsV1().CustomResourceDefinitions()
				require.Eventually(t, func() bool {
					_, err := crdClient.Get(ctx, serviceGVR.Resource+"."+serviceGVR.Group, metav1.GetOptions{})
					return err == nil
				}, wait.ForeverTestTimeout, time.Millisecond*100, "waiting for %s CRD to be created on consumer side", serviceGVR.Resource)
			},
		},
		{
			name: "instances are synced",
			step: func(t *testing.T) {
				t.Logf("Trying to create %s on consumer side", serviceGVR.Resource)

				require.Eventually(t, func() bool {
					var err error
					if resourceScope == apiextensionsv1.NamespaceScoped {
						_, err = consumerClient.Namespace(downstreamNs).Create(ctx, toUnstructured(t, mangodbInstance), metav1.CreateOptions{})
					} else {
						_, err = consumerClient.Create(ctx, toUnstructured(t, fooInstance), metav1.CreateOptions{})
					}
					return err == nil
				}, wait.ForeverTestTimeout, time.Millisecond*100, "waiting for %s instance to be created on consumer side", serviceGVR.Resource)

				t.Logf("Waiting for the %s instance to be created on provider side", serviceGVR.Resource)
				var instances *unstructured.UnstructuredList
				require.Eventually(t, func() bool {
					var err error
					instances, err = providerClient.List(ctx, metav1.ListOptions{})
					return err == nil && len(instances.Items) == 1
				}, wait.ForeverTestTimeout, time.Millisecond*100, "waiting for the %s instance to be created on provider side", serviceGVR.Resource)

				// these are used everywhere further down
				upstreamNS = instances.Items[0].GetNamespace()
				if resourceScope == apiextensionsv1.ClusterScoped {
					clusterNs, _ = clusterscoped.ExtractClusterNs(&instances.Items[0])
					clusterScopedUpInsName = clusterscoped.Prepend("test", clusterNs)
				}
			},
		},
		{
			name: "create secrets and configmaps if permission claims enabled",
			step: func(t *testing.T) {
				if !withPermissionClaims {
					t.Skip("Skipping permission claims test when permission claims are disabled")
					return
				}

				t.Logf("Creating configmap on consumer side")
				configMapData := map[string]string{
					"config.yaml": "test: value",
				}
				configMap := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-configmap",
						Namespace: downstreamNs,
						Labels: map[string]string{
							"app": "configmaps",
						},
					},
					Data: configMapData,
				}
				_, err := consumerCoreClient.ConfigMaps(downstreamNs).Create(ctx, configMap, metav1.CreateOptions{})
				require.NoError(t, err)

				t.Logf("Creating secret on consumer side")
				secretData := map[string][]byte{
					"password": []byte("secret-password"),
				}
				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-secret",
						Namespace: downstreamNs,
						Labels: map[string]string{
							"app": "secrets",
						},
					},
					Data: secretData,
				}
				_, err = consumerCoreClient.Secrets(downstreamNs).Create(ctx, secret, metav1.CreateOptions{})
				require.NoError(t, err)
			},
		},
		{
			name: "verify secrets and configmaps are synced to provider",
			step: func(t *testing.T) {
				if !withPermissionClaims {
					t.Skip("Skipping permission claims test when permission claims are disabled")
					return
				}

				t.Logf("Waiting for configmap to be synced to provider side")
				require.Eventually(t, func() bool {
					_, err := providerCoreClient.ConfigMaps(upstreamNS).Get(ctx, "test-configmap", metav1.GetOptions{})
					return err == nil
				}, wait.ForeverTestTimeout, time.Millisecond*100, "waiting for configmap to be synced to provider side")

				t.Logf("Waiting for secret to be synced to provider side")
				require.Eventually(t, func() bool {
					_, err := providerCoreClient.Secrets(upstreamNS).Get(ctx, "test-secret", metav1.GetOptions{})
					return err == nil
				}, wait.ForeverTestTimeout, time.Millisecond*100, "waiting for secret to be synced to provider side")

				t.Logf("Verifying configmap data is correct")
				providerConfigMap, err := providerCoreClient.ConfigMaps(upstreamNS).Get(ctx, "test-configmap", metav1.GetOptions{})
				require.NoError(t, err)
				require.Equal(t, "test: value", providerConfigMap.Data["config.yaml"])

				t.Logf("Verifying secret data is correct")
				providerSecret, err := providerCoreClient.Secrets(upstreamNS).Get(ctx, "test-secret", metav1.GetOptions{})
				require.NoError(t, err)
				require.Equal(t, []byte("secret-password"), providerSecret.Data["password"])
			},
		},
		{
			name: "verify secrets and configmaps are deleted when removed from consumer",
			step: func(t *testing.T) {
				if !withPermissionClaims {
					t.Skip("Skipping permission claims test when permission claims are disabled")
					return
				}

				t.Logf("Deleting configmap from consumer side")
				err := consumerCoreClient.ConfigMaps(downstreamNs).Delete(ctx, "test-configmap", metav1.DeleteOptions{})
				require.NoError(t, err)

				t.Logf("Deleting secret from consumer side")
				err = consumerCoreClient.Secrets(downstreamNs).Delete(ctx, "test-secret", metav1.DeleteOptions{})
				require.NoError(t, err)

				t.Logf("Waiting for configmap to be deleted from provider side")
				require.Eventually(t, func() bool {
					_, err := providerCoreClient.ConfigMaps(upstreamNS).Get(ctx, "test-configmap", metav1.GetOptions{})
					return errors.IsNotFound(err)
				}, wait.ForeverTestTimeout, time.Millisecond*100, "waiting for configmap to be deleted from provider side")

				t.Logf("Waiting for secret to be deleted from provider side")
				require.Eventually(t, func() bool {
					_, err := providerCoreClient.Secrets(upstreamNS).Get(ctx, "test-secret", metav1.GetOptions{})
					return errors.IsNotFound(err)
				}, wait.ForeverTestTimeout, time.Millisecond*100, "waiting for secret to be deleted from provider side")
			},
		},
		{
			name: "instance deleted upstream is recreated",
			step: func(t *testing.T) {
				var err error
				if resourceScope == apiextensionsv1.NamespaceScoped {
					err = providerClient.Namespace(upstreamNS).Delete(ctx, "test", metav1.DeleteOptions{})
				} else {
					err = providerClient.Delete(ctx, clusterScopedUpInsName, metav1.DeleteOptions{})
				}
				require.NoError(t, err)

				require.Eventually(t, func() bool {
					var err error
					if resourceScope == apiextensionsv1.NamespaceScoped {
						_, err = providerClient.Namespace(upstreamNS).Get(ctx, "test", metav1.GetOptions{})
					} else {
						_, err = providerClient.Get(ctx, clusterScopedUpInsName, metav1.GetOptions{})
					}
					return err == nil
				}, wait.ForeverTestTimeout, time.Millisecond*100, "waiting for the %s instance to be recreated upstream", serviceGVR.Resource)
			},
		},
		{
			name: "instance spec updated downstream is updated upstream",
			step: func(t *testing.T) {
				err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
					var obj *unstructured.Unstructured
					var err error
					if resourceScope == apiextensionsv1.NamespaceScoped {
						obj, err = consumerClient.Namespace(downstreamNs).Get(ctx, "test", metav1.GetOptions{})
					} else {
						obj, err = consumerClient.Get(ctx, "test", metav1.GetOptions{})
					}
					require.NoError(t, err)
					if resourceScope == apiextensionsv1.NamespaceScoped {
						unstructured.SetNestedField(obj.Object, "Dedicated", "spec", "tier") //nolint:errcheck
						_, err = consumerClient.Namespace(downstreamNs).Update(ctx, obj, metav1.UpdateOptions{})
					} else {
						unstructured.SetNestedField(obj.Object, "tested", "spec", "deploymentName") //nolint:errcheck
						_, err = consumerClient.Update(ctx, obj, metav1.UpdateOptions{})
					}
					return err
				})
				require.NoError(t, err)

				require.Eventually(t, func() bool {
					var obj *unstructured.Unstructured
					var err error
					if resourceScope == apiextensionsv1.NamespaceScoped {
						obj, err = providerClient.Namespace(upstreamNS).Get(ctx, "test", metav1.GetOptions{})
					} else {
						obj, err = providerClient.Get(ctx, clusterScopedUpInsName, metav1.GetOptions{})
					}
					require.NoError(t, err)
					var value string
					if resourceScope == apiextensionsv1.NamespaceScoped {
						value, _, err = unstructured.NestedString(obj.Object, "spec", "tier")
					} else {
						value, _, err = unstructured.NestedString(obj.Object, "spec", "deploymentName")
					}
					require.NoError(t, err)
					if resourceScope == apiextensionsv1.NamespaceScoped {
						return value == "Dedicated"
					} else {
						return value == "tested"
					}
				}, wait.ForeverTestTimeout, time.Millisecond*100, "waiting for the %s instance to be updated upstream", serviceGVR.Resource)
			},
		},
		{
			name: "instance status updated upstream is updated downstream",
			step: func(t *testing.T) {
				err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
					var obj *unstructured.Unstructured
					var err error
					if resourceScope == apiextensionsv1.NamespaceScoped {
						obj, err = providerClient.Namespace(upstreamNS).Get(ctx, "test", metav1.GetOptions{})
					} else {
						obj, err = providerClient.Get(ctx, clusterScopedUpInsName, metav1.GetOptions{})
					}
					require.NoError(t, err)
					unstructured.SetNestedField(obj.Object, "Running", "status", "phase") //nolint:errcheck
					if resourceScope == apiextensionsv1.NamespaceScoped {
						_, err = providerClient.Namespace(upstreamNS).UpdateStatus(ctx, obj, metav1.UpdateOptions{})
					} else {
						_, err = providerClient.UpdateStatus(ctx, obj, metav1.UpdateOptions{})
					}
					return err
				})
				require.NoError(t, err)

				require.Eventually(t, func() bool {
					var obj *unstructured.Unstructured
					var err error
					if resourceScope == apiextensionsv1.NamespaceScoped {
						obj, err = consumerClient.Namespace(downstreamNs).Get(ctx, "test", metav1.GetOptions{})
					} else {
						obj, err = consumerClient.Get(ctx, "test", metav1.GetOptions{})
					}
					require.NoError(t, err)
					value, _, err := unstructured.NestedString(obj.Object, "status", "phase")
					require.NoError(t, err)
					return value == "Running"
				}, wait.ForeverTestTimeout, time.Millisecond*100, "waiting for the %s instance to be updated downstream", serviceGVR.Resource)
			},
		},
		{
			name: "instance spec updated upstream is reconciled upstream",
			step: func(t *testing.T) {
				err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
					var obj *unstructured.Unstructured
					var err error
					if resourceScope == apiextensionsv1.NamespaceScoped {
						obj, err = providerClient.Namespace(upstreamNS).Get(ctx, "test", metav1.GetOptions{})
					} else {
						obj, err = providerClient.Get(ctx, clusterScopedUpInsName, metav1.GetOptions{})
					}
					require.NoError(t, err)
					if resourceScope == apiextensionsv1.NamespaceScoped {
						unstructured.SetNestedField(obj.Object, "Shared", "spec", "tier") //nolint:errcheck
						_, err = providerClient.Namespace(upstreamNS).Update(ctx, obj, metav1.UpdateOptions{})
					} else {
						unstructured.SetNestedField(obj.Object, "drifting", "spec", "deploymentName") //nolint:errcheck
						_, err = providerClient.Update(ctx, obj, metav1.UpdateOptions{})
					}
					return err
				})
				require.NoError(t, err)

				require.Eventually(t, func() bool {
					var obj *unstructured.Unstructured
					var err error
					if resourceScope == apiextensionsv1.NamespaceScoped {
						obj, err = providerClient.Namespace(upstreamNS).Get(ctx, "test", metav1.GetOptions{})
					} else {
						obj, err = providerClient.Get(ctx, clusterScopedUpInsName, metav1.GetOptions{})
					}
					require.NoError(t, err)
					var value string
					if resourceScope == apiextensionsv1.NamespaceScoped {
						value, _, err = unstructured.NestedString(obj.Object, "spec", "tier")
					} else {
						value, _, err = unstructured.NestedString(obj.Object, "spec", "deploymentName")
					}
					require.NoError(t, err)
					if resourceScope == apiextensionsv1.NamespaceScoped {
						return value == "Dedicated"
					} else {
						return value == "tested"
					}
				}, wait.ForeverTestTimeout, time.Millisecond*100, "waiting for the %s instance to be reconciled upstream", serviceGVR.Resource)
			},
		},
		{
			name: "instances deleted downstream are deleted upstream",
			step: func(t *testing.T) {
				var err error
				if resourceScope == apiextensionsv1.NamespaceScoped {
					err = consumerClient.Namespace(downstreamNs).Delete(ctx, "test", metav1.DeleteOptions{})
				} else {
					err = consumerClient.Delete(ctx, "test", metav1.DeleteOptions{})
				}
				require.NoError(t, err)

				require.Eventually(t, func() bool {
					var err error
					if resourceScope == apiextensionsv1.NamespaceScoped {
						_, err = providerClient.Namespace(upstreamNS).Get(ctx, "test", metav1.GetOptions{})
					} else {
						_, err = providerClient.Get(ctx, clusterScopedUpInsName, metav1.GetOptions{})
					}
					return errors.IsNotFound(err)
				}, wait.ForeverTestTimeout, time.Millisecond*100, "waiting for the %s instance to be deleted on provider side", serviceGVR.Resource)
			},
		},
		{
			name: "Bind again",
			step: func(t *testing.T) {
				iostreams, _, _, _ := genericclioptions.NewTestIOStreams()
				authURLCh := make(chan string, 1)
				go simulateBrowser(t, authURLCh, serviceGVR.Resource)
				invocations := make(chan framework.SubCommandInvocation, 1)
				framework.Bind(t, iostreams, authURLCh, invocations, fmt.Sprintf("http://%s/exports", addr.String()), "--kubeconfig", consumerKubeconfig, "--skip-konnector")
				inv := <-invocations
				requireEqualSlicePattern(t, []string{"apiservice", "--remote-kubeconfig-namespace", "*", "--remote-kubeconfig-name", "*", "-f", "-", "--kubeconfig=" + consumerKubeconfig, "--skip-konnector=true", "--no-banner"}, inv.Args)
				framework.BindAPIService(t, inv.Stdin, "", inv.Args...)
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.step(t)
		})
	}
}

func simulateBrowser(t *testing.T, authURLCh chan string, resource string) {
	browser := surf.NewBrowser()
	authURL := <-authURLCh

	t.Logf("Browsing to auth URL: %s", authURL)
	err := browser.Open(authURL)
	require.NoError(t, err)

	t.Logf("Waiting for browser to be at /resources")
	framework.BrowserEventuallyAtPath(t, browser, "/resources")

	t.Logf("Clicking %s", resource)
	err = browser.Click("a." + resource)
	require.NoError(t, err)

	t.Logf("Waiting for browser to be forwarded to client")
	framework.BrowserEventuallyAtPath(t, browser, "/callback")
}

func toUnstructured(t *testing.T, manifest string) *unstructured.Unstructured {
	t.Helper()

	obj := map[string]any{}
	err := yaml.Unmarshal([]byte(manifest), &obj)
	require.NoError(t, err)

	return &unstructured.Unstructured{Object: obj}
}

func requireEqualSlicePattern(t *testing.T, pattern []string, slice []string) {
	t.Helper()

	require.Equal(t, len(pattern), len(slice), "slice length doesn't match pattern length\n     got: %s\nexpected: %s", strings.Join(slice, " "), strings.Join(pattern, " "))

	for i, s := range slice {
		if pattern[i] == "*" {
			continue
		}
		require.Equal(t, pattern[i], s, "slice doesn't match pattern at index %d\n     got: %s\nexpected: %s", i, strings.Join(slice, " "), strings.Join(pattern, " "))
	}
}
