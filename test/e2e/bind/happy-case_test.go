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
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gopkg.in/headzoo/surf.v1"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/yaml"

	kubebindv1alpha1 "github.com/kube-bind/kube-bind/pkg/apis/kubebind/v1alpha1"
	clusterscoped "github.com/kube-bind/kube-bind/pkg/konnector/controllers/cluster/serviceexport/cluster-scoped"
	providerfixtures "github.com/kube-bind/kube-bind/test/e2e/bind/fixtures/provider"
	"github.com/kube-bind/kube-bind/test/e2e/framework"
)

func TestClusterScoped(t *testing.T) {
	t.Parallel()

	// cluster scoped resource, with cluster scoped informers
	testHappyCase(t, apiextensionsv1.ClusterScoped, kubebindv1alpha1.ClusterScope)
}

func TestNamespacedScoped(t *testing.T) {
	t.Parallel()

	// namespaced resource, with namespace scoped informers
	testHappyCase(t, apiextensionsv1.NamespaceScoped, kubebindv1alpha1.NamespacedScope)
	// namespaced resource, but with cluster scoped informers
	testHappyCase(t, apiextensionsv1.NamespaceScoped, kubebindv1alpha1.ClusterScope)
}

func testHappyCase(t *testing.T, resourceScope apiextensionsv1.ResourceScope, informerScope kubebindv1alpha1.Scope) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	t.Logf("Creating provider workspace")
	providerConfig, providerKubeconfig := framework.NewWorkspace(t, framework.ClientConfig(t), framework.WithGenerateName("test-happy-case-provider"))

	t.Logf("Creating CRDs on provider side")
	providerfixtures.Bootstrap(t, framework.DiscoveryClient(t, providerConfig), framework.DynamicClient(t, providerConfig), nil)

	t.Logf("Starting backend with random port")
	addr, _ := framework.StartBackend(t, providerConfig, "--kubeconfig="+providerKubeconfig, "--listen-port=0", "--consumer-scope="+string(informerScope))

	t.Logf("Creating consumer workspace and starting konnector")
	consumerConfig, consumerKubeconfig := framework.NewWorkspace(t, framework.ClientConfig(t), framework.WithGenerateName("test-happy-case-consumer"))
	framework.StartKonnector(t, consumerConfig, "--kubeconfig="+consumerKubeconfig)

	serviceGVR := schema.GroupVersionResource{Group: "mangodb.com", Version: "v1alpha1", Resource: "mangodbs"}
	if resourceScope == apiextensionsv1.ClusterScoped {
		serviceGVR = schema.GroupVersionResource{Group: "bar.io", Version: "v1alpha1", Resource: "foos"}
	}
	consumerClient := framework.DynamicClient(t, consumerConfig).Resource(serviceGVR)
	providerClient := framework.DynamicClient(t, providerConfig).Resource(serviceGVR)

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
				framework.Bind(t, iostreams, authURLDryRunCh, nil, fmt.Sprintf("http://%s/export", addr.String()), "--kubeconfig", consumerKubeconfig, "--skip-konnector", "--dry-run")
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
				framework.Bind(t, iostreams, authURLCh, invocations, fmt.Sprintf("http://%s/export", addr.String()), "--kubeconfig", consumerKubeconfig, "--skip-konnector")
				inv := <-invocations
				requireEqualSlicePattern(t, []string{"apiservice", "--remote-kubeconfig-namespace", "*", "--remote-kubeconfig-name", "*", "-f", "-", "--kubeconfig=" + consumerKubeconfig, "--skip-konnector=true", "--no-banner"}, inv.Args)
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
						unstructured.SetNestedField(obj.Object, "Dedicated", "spec", "tier") // nolint: errcheck
						_, err = consumerClient.Namespace(downstreamNs).Update(ctx, obj, metav1.UpdateOptions{})
					} else {
						unstructured.SetNestedField(obj.Object, "tested", "spec", "deploymentName") // nolint: errcheck
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
					unstructured.SetNestedField(obj.Object, "Running", "status", "phase") // nolint: errcheck
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
						unstructured.SetNestedField(obj.Object, "Shared", "spec", "tier") // nolint: errcheck
						_, err = providerClient.Namespace(upstreamNS).Update(ctx, obj, metav1.UpdateOptions{})
					} else {
						unstructured.SetNestedField(obj.Object, "drifting", "spec", "deploymentName") // nolint: errcheck
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
				framework.Bind(t, iostreams, authURLCh, invocations, fmt.Sprintf("http://%s/export", addr.String()), "--kubeconfig", consumerKubeconfig, "--skip-konnector")
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
	framework.BrowerEventuallyAtPath(t, browser, "/resources")

	t.Logf("Clicking %s", resource)
	err = browser.Click("a." + resource)
	require.NoError(t, err)

	t.Logf("Waiting for browser to be forwarded to client")
	framework.BrowerEventuallyAtPath(t, browser, "/callback")
}

func toUnstructured(t *testing.T, manifest string) *unstructured.Unstructured {
	t.Helper()

	obj := map[string]interface{}{}
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
