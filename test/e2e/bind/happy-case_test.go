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
	"bytes"
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gopkg.in/headzoo/surf.v1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/yaml"

	kubebindv1alpha1 "github.com/kube-bind/kube-bind/pkg/apis/kubebind/v1alpha1"
	providerfixtures "github.com/kube-bind/kube-bind/test/e2e/bind/fixtures/provider"
	"github.com/kube-bind/kube-bind/test/e2e/framework"
)

func TestClusterScoped(t *testing.T) {
	t.Parallel()

	testHappyCase(t, kubebindv1alpha1.ClusterScope)
}

func TestNamespacedScoped(t *testing.T) {
	t.Parallel()

	testHappyCase(t, kubebindv1alpha1.NamespacedScope)
}

func testHappyCase(t *testing.T, scope kubebindv1alpha1.Scope) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	t.Logf("Creating provider workspace")
	providerConfig, providerKubeconfig := framework.NewWorkspace(t, framework.ClientConfig(t), framework.WithGenerateName("test-happy-case-provider"))

	t.Logf("Starting backend with random port")
	addr, _ := framework.StartBackend(t, providerConfig, "--kubeconfig="+providerKubeconfig, "--listen-port=0", "--consumer-scope="+string(scope))

	t.Logf("Creating MangoDB CRD on provider side")
	providerfixtures.Bootstrap(t, framework.DiscoveryClient(t, providerConfig), framework.DynamicClient(t, providerConfig), nil)

	t.Logf("Creating consumer workspace and starting konnector")
	consumerConfig, consumerKubeconfig := framework.NewWorkspace(t, framework.ClientConfig(t), framework.WithGenerateName("test-happy-case-consumer"))
	framework.StartKonnector(t, consumerConfig, "--kubeconfig="+consumerKubeconfig)

	consumerClient := framework.DynamicClient(t, consumerConfig).Resource(
		schema.GroupVersionResource{Group: "mangodb.com", Version: "v1alpha1", Resource: "mangodbs"},
	).Namespace("default")
	providerClient := framework.DynamicClient(t, providerConfig).Resource(
		schema.GroupVersionResource{Group: "mangodb.com", Version: "v1alpha1", Resource: "mangodbs"},
	)
	providerKubeClient := framework.KubeClient(t, providerConfig)
	consumerKubeClient := framework.KubeClient(t, consumerConfig)
	upstreamNS := "unknown"
	downstreamNS := "unknown"

	for _, tc := range []struct {
		name string
		step func(t *testing.T)
	}{
		{
			name: "MangoDB is bound dry run",
			step: func(t *testing.T) {
				iostreams, _, bufOut, _ := genericclioptions.NewTestIOStreams()
				authURLDryRunCh := make(chan string, 1)
				go simulateBrowser(t, authURLDryRunCh, "mangodbs")
				framework.Bind(t, iostreams, authURLDryRunCh, nil, fmt.Sprintf("http://%s/export", addr.String()), "--kubeconfig", consumerKubeconfig, "--skip-konnector", "--dry-run")
				_, err := yaml.YAMLToJSON(bufOut.Bytes())
				require.NoError(t, err)
			},
		},
		{
			name: "MangoDB is bound",
			step: func(t *testing.T) {
				in := bytes.NewBufferString("y\n")
				iostreams, _, _, _ := genericclioptions.NewTestIOStreams()
				authURLCh := make(chan string, 1)
				go simulateBrowser(t, authURLCh, "mangodbs")
				invocations := make(chan framework.SubCommandInvocation, 1)
				framework.Bind(t, iostreams, authURLCh, invocations, fmt.Sprintf("http://%s/export", addr.String()), "--kubeconfig", consumerKubeconfig, "--skip-konnector")
				inv := <-invocations
				requireEqualSlicePattern(t, []string{"apiservice", "--remote-kubeconfig-namespace", "*", "--remote-kubeconfig-name", "*", "-f", "*", "--kubeconfig=" + consumerKubeconfig, "--skip-konnector=true", "--no-banner"}, inv.Args)
				framework.BindAPIService(t, in, "", inv.Args...)

				t.Logf("Waiting for MangoDB CRD to be created on consumer side")
				crdClient := framework.ApiextensionsClient(t, consumerConfig).ApiextensionsV1().CustomResourceDefinitions()
				require.Eventually(t, func() bool {
					_, err := crdClient.Get(ctx, "mangodbs.mangodb.com", metav1.GetOptions{})
					return err == nil
				}, wait.ForeverTestTimeout, time.Millisecond*100, "waiting for MangoDB CRD to be created on consumer side")
			},
		},
		{
			name: "instances are synced",
			step: func(t *testing.T) {
				t.Logf("Trying to create MangoDB on consumer side")

				require.Eventually(t, func() bool {
					_, err := consumerClient.Create(ctx, toUnstructured(t, `
apiVersion: mangodb.com/v1alpha1
kind: MangoDB
metadata:
  name: test
spec:
  tokenSecret: credentials
`), metav1.CreateOptions{})
					return err == nil
				}, wait.ForeverTestTimeout, time.Millisecond*100, "waiting for MangoDB CRD to be created on consumer side")

				t.Logf("Waiting for the MangoDB instance to be created on consumer side")
				var consumerMangos *unstructured.UnstructuredList
				require.Eventually(t, func() bool {
					var err error
					consumerMangos, err = consumerClient.List(ctx, metav1.ListOptions{})
					return err == nil && len(consumerMangos.Items) == 1
				}, wait.ForeverTestTimeout, time.Millisecond*100, "waiting for the MangoDB instance to be created on consumer side")

				// this is used everywhere further down
				downstreamNS = consumerMangos.Items[0].GetNamespace()

				t.Logf("Waiting for the MangoDB instance to be created on provider side")
				var mangos *unstructured.UnstructuredList
				require.Eventually(t, func() bool {
					var err error
					mangos, err = providerClient.List(ctx, metav1.ListOptions{})
					return err == nil && len(mangos.Items) == 1
				}, wait.ForeverTestTimeout, time.Millisecond*100, "waiting for the MangoDB instance to be created on provider side")

				// this is used everywhere further down
				upstreamNS = mangos.Items[0].GetNamespace()
			},
		},
		{
			name: "instance deleted upstream is recreated",
			step: func(t *testing.T) {
				err := providerClient.Namespace(upstreamNS).Delete(ctx, "test", metav1.DeleteOptions{})
				require.NoError(t, err)

				require.Eventually(t, func() bool {
					_, err := providerClient.Namespace(upstreamNS).Get(ctx, "test", metav1.GetOptions{})
					return err == nil
				}, wait.ForeverTestTimeout, time.Millisecond*100, "waiting for the MangoDB instance to be recreated upstream")
			},
		},
		{
			name: "claimed resource created upstream is created downstream",
			step: func(t *testing.T) {
				testSecret := corev1.Secret{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Secret",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-secret",
						Namespace: upstreamNS,
					},
					Data: map[string][]byte{
						"test": []byte("dummy"),
					},
				}

				_, err := providerKubeClient.CoreV1().Secrets(upstreamNS).Create(ctx, &testSecret, metav1.CreateOptions{})
				require.NoError(t, err)

				require.Eventually(t, func() bool {
					s, err := consumerKubeClient.CoreV1().Secrets(downstreamNS).Get(ctx, "test-secret", metav1.GetOptions{})
					return err == nil && reflect.DeepEqual(testSecret.Data, s.Data)
				}, wait.ForeverTestTimeout, time.Millisecond*100, "waiting for the claimed resource to be created downstream")
			},
		},
		{
			name: "claimed resource recreated downstream if created upstream",
			step: func(t *testing.T) {
				err := consumerKubeClient.CoreV1().Secrets(downstreamNS).Delete(ctx, "test-secret", metav1.DeleteOptions{})
				require.NoError(t, err)

				require.Eventually(t, func() bool {
					_, err := consumerKubeClient.CoreV1().Secrets(downstreamNS).Get(ctx, "test-secret", metav1.GetOptions{})
					return err == nil
				}, wait.ForeverTestTimeout, time.Millisecond*100, "waiting for the claimed resource to be created downstream")
			},
		},
		{
			name: "claimed resource updated upstream is updated downstream",
			step: func(t *testing.T) {
				err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
					obj, err := providerKubeClient.CoreV1().Secrets(upstreamNS).Get(ctx, "test-secret", metav1.GetOptions{})
					require.NoError(t, err)
					obj.Data["test"] = []byte("updated")
					_, err = providerKubeClient.CoreV1().Secrets(upstreamNS).Update(ctx, obj, metav1.UpdateOptions{})
					return err
				})
				require.NoError(t, err)

				require.Eventually(t, func() bool {
					obj, err := consumerKubeClient.CoreV1().Secrets(downstreamNS).Get(ctx, "test-secret", metav1.GetOptions{})
					require.NoError(t, err)
					updatedValue, ok := obj.Data["test"]
					if !ok {
						return false
					}

					return string(updatedValue) == "updated"
				}, wait.ForeverTestTimeout, time.Millisecond*100, "waiting for claimed secret to be updated downstream")
			},
		},
		{
			name: "claimed resources deleted by the provider are deleted downstream",
			step: func(t *testing.T) {
				err := providerKubeClient.CoreV1().Secrets(upstreamNS).Delete(ctx, "test-secret", metav1.DeleteOptions{})
				require.NoError(t, err)

				require.Eventually(t, func() bool {
					_, err := consumerKubeClient.CoreV1().Secrets(upstreamNS).Get(ctx, "test-secret", metav1.GetOptions{})
					return errors.IsNotFound(err)
				}, wait.ForeverTestTimeout, time.Millisecond*100, "waiting for claimed secret to be deleted on consumer side")
			},
		},
		{
			name: "instance spec updated downstream is updated upstream",
			step: func(t *testing.T) {
				err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
					obj, err := consumerClient.Get(ctx, "test", metav1.GetOptions{})
					require.NoError(t, err)
					unstructured.SetNestedField(obj.Object, "Dedicated", "spec", "tier") // nolint: errcheck
					_, err = consumerClient.Update(ctx, obj, metav1.UpdateOptions{})
					return err
				})
				require.NoError(t, err)

				require.Eventually(t, func() bool {
					obj, err := providerClient.Namespace(upstreamNS).Get(ctx, "test", metav1.GetOptions{})
					require.NoError(t, err)
					value, _, err := unstructured.NestedString(obj.Object, "spec", "tier")
					require.NoError(t, err)
					return value == "Dedicated"
				}, wait.ForeverTestTimeout, time.Millisecond*100, "waiting for the MangoDB instance to be updated upstream")
			},
		},
		{
			name: "instance status updated upstream is updated downstream",
			step: func(t *testing.T) {
				err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
					obj, err := providerClient.Namespace(upstreamNS).Get(ctx, "test", metav1.GetOptions{})
					require.NoError(t, err)
					unstructured.SetNestedField(obj.Object, "Running", "status", "phase") // nolint: errcheck
					_, err = providerClient.Namespace(upstreamNS).UpdateStatus(ctx, obj, metav1.UpdateOptions{})
					return err
				})
				require.NoError(t, err)

				require.Eventually(t, func() bool {
					obj, err := consumerClient.Get(ctx, "test", metav1.GetOptions{})
					require.NoError(t, err)
					value, _, err := unstructured.NestedString(obj.Object, "status", "phase")
					require.NoError(t, err)
					return value == "Running"
				}, wait.ForeverTestTimeout, time.Millisecond*100, "waiting for the MangoDB instance to be updated downstream")
			},
		},
		{
			name: "instance spec updated upstream is reconciled upstream",
			step: func(t *testing.T) {
				err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
					obj, err := providerClient.Namespace(upstreamNS).Get(ctx, "test", metav1.GetOptions{})
					require.NoError(t, err)
					unstructured.SetNestedField(obj.Object, "Shared", "spec", "tier") // nolint: errcheck
					_, err = providerClient.Namespace(upstreamNS).Update(ctx, obj, metav1.UpdateOptions{})
					return err
				})
				require.NoError(t, err)

				require.Eventually(t, func() bool {
					obj, err := providerClient.Namespace(upstreamNS).Get(ctx, "test", metav1.GetOptions{})
					require.NoError(t, err)
					value, _, err := unstructured.NestedString(obj.Object, "spec", "tier")
					require.NoError(t, err)
					return value == "Dedicated"
				}, wait.ForeverTestTimeout, time.Millisecond*100, "waiting for the MangoDB instance to be reconciled upstream")
			},
		},
		{
			name: "instances deleted downstream are deleted upstream",
			step: func(t *testing.T) {
				err := consumerClient.Delete(ctx, "test", metav1.DeleteOptions{})
				require.NoError(t, err)

				require.Eventually(t, func() bool {
					_, err := providerClient.Namespace(upstreamNS).Get(ctx, "test", metav1.GetOptions{})
					return errors.IsNotFound(err)
				}, wait.ForeverTestTimeout, time.Millisecond*100, "waiting for the MangoDB instance to be deleted on provider side")
			},
		},
		{
			name: "Bind again",
			step: func(t *testing.T) {
				iostreams, _, _, _ := genericclioptions.NewTestIOStreams()
				authURLCh := make(chan string, 1)
				go simulateBrowser(t, authURLCh, "mangodbs")
				invocations := make(chan framework.SubCommandInvocation, 1)
				framework.Bind(t, iostreams, authURLCh, invocations, fmt.Sprintf("http://%s/export", addr.String()), "--kubeconfig", consumerKubeconfig, "--skip-konnector")
				inv := <-invocations
				requireEqualSlicePattern(t, []string{"apiservice", "--remote-kubeconfig-namespace", "*", "--remote-kubeconfig-name", "*", "-f", "*", "--kubeconfig=" + consumerKubeconfig, "--skip-konnector=true", "--no-banner"}, inv.Args)
				framework.BindAPIService(t, bytes.NewBufferString("y\n"), "", inv.Args...)
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
