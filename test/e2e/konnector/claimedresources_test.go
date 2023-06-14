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

package konnector

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gopkg.in/headzoo/surf.v1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"sigs.k8s.io/yaml"

	kubebindv1alpha1 "github.com/kube-bind/kube-bind/pkg/apis/kubebind/v1alpha1"
	consumerfixtures "github.com/kube-bind/kube-bind/test/e2e/bind/fixtures/consumer"
	providerfixtures "github.com/kube-bind/kube-bind/test/e2e/bind/fixtures/provider"
	"github.com/kube-bind/kube-bind/test/e2e/framework"
)

func TestProviderOwned(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	t.Logf("Creating provider workspace")
	providerConfig, providerKubeconfig := framework.NewWorkspace(t, framework.ClientConfig(t), framework.WithGenerateName("test-claimed-resources-provider"))

	t.Logf("Starting backend with random port")
	addr, _ := framework.StartBackend(t, providerConfig, "--kubeconfig="+providerKubeconfig, "--listen-port=0", "--consumer-scope="+string(kubebindv1alpha1.NamespacedScope))

	t.Logf("Creating MangoDB CRD on provider side")
	providerfixtures.Bootstrap(t, framework.DiscoveryClient(t, providerConfig), framework.DynamicClient(t, providerConfig), nil)

	t.Logf("Creating consumer workspace and starting konnector")
	consumerConfig, consumerKubeconfig := framework.NewWorkspace(t, framework.ClientConfig(t), framework.WithGenerateName("test-claimed-resources-provider"))
	framework.StartKonnector(t, consumerConfig, "--kubeconfig="+consumerKubeconfig)

	providerKubeClient := framework.KubeClient(t, providerConfig)
	consumerKubeClient := framework.KubeClient(t, consumerConfig)

	consumerClient := framework.DynamicClient(t, consumerConfig).Resource(
		schema.GroupVersionResource{Group: "mangodb.com", Version: "v1alpha1", Resource: "mangodbs"},
	).Namespace("default")
	providerClient := framework.DynamicClient(t, providerConfig).Resource(
		schema.GroupVersionResource{Group: "mangodb.com", Version: "v1alpha1", Resource: "mangodbs"},
	)

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
			name: "secret creation at provider",
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

			},
		},
		{
			name: "secret is automatically created at consumer",
			step: func(t *testing.T) {
				require.Eventually(t, func() bool {
					_, err := consumerKubeClient.CoreV1().Secrets(downstreamNS).Get(ctx, "test-secret", metav1.GetOptions{})
					return err == nil
				}, wait.ForeverTestTimeout, time.Millisecond*100, "waiting for secret to be created at consumer")
			},
		},
		{
			name: "secret updated at consumer is overwritten",
			step: func(t *testing.T) {
				require.Eventually(t, func() bool {
					s, err := consumerKubeClient.CoreV1().Secrets(downstreamNS).Get(ctx, "test-secret", metav1.GetOptions{})
					if err != nil {
						return false
					}

					s.Data["test"] = []byte("updated")

					_, err = consumerKubeClient.CoreV1().Secrets(downstreamNS).Update(ctx, s, metav1.UpdateOptions{})
					return err == nil
				}, wait.ForeverTestTimeout, time.Millisecond*100, "waiting for secret to be updated at consumer")

				require.Eventually(t, func() bool {
					s, err := consumerKubeClient.CoreV1().Secrets(downstreamNS).Get(ctx, "test-secret", metav1.GetOptions{})
					if err != nil {
						return false
					}

					if v, ok := s.Data["test"]; ok && equality.Semantic.DeepEqual(v, []byte("dummy")) {
						return true
					}
					return false
				}, wait.ForeverTestTimeout, time.Millisecond*100, "waiting for secret to be overwritten at consumer")
			},
		},
		{
			name: "secret is automatically updated at consumer",
			step: func(t *testing.T) {
				require.Eventually(t, func() bool {
					s, err := providerKubeClient.CoreV1().Secrets(upstreamNS).Get(ctx, "test-secret", metav1.GetOptions{})
					if err != nil {
						return false
					}

					s.Data["test"] = []byte("updated")

					_, err = providerKubeClient.CoreV1().Secrets(upstreamNS).Update(ctx, s, metav1.UpdateOptions{})
					return err == nil
				}, wait.ForeverTestTimeout, time.Millisecond*100, "waiting for secret to be updated at provider")

				require.Eventually(t, func() bool {
					s, err := consumerKubeClient.CoreV1().Secrets(downstreamNS).Get(ctx, "test-secret", metav1.GetOptions{})
					if err != nil {
						return false
					}

					if v, ok := s.Data["test"]; ok && equality.Semantic.DeepEqual(v, []byte("updated")) {
						return true
					}
					return false
				}, wait.ForeverTestTimeout, time.Millisecond*100, "waiting for secret to be updated at consumer")
			},
		},
		{
			name: "secret deleted at consumer is automatically recreated",
			step: func(t *testing.T) {
				require.Eventually(t, func() bool {
					err := consumerKubeClient.CoreV1().Secrets(downstreamNS).Delete(ctx, "test-secret", metav1.DeleteOptions{})
					return err == nil
				}, wait.ForeverTestTimeout, time.Millisecond*100, "waiting for secret to be deleted at consumer")

				require.Eventually(t, func() bool {
					_, err := consumerKubeClient.CoreV1().Secrets(downstreamNS).Get(ctx, "test-secret", metav1.GetOptions{})
					return !errors.IsNotFound(err)
				}, wait.ForeverTestTimeout, time.Millisecond*100, "waiting for secret to be recreated at consumer")
			},
		},
		{
			name: "secret is automatically deleted at consumer",
			step: func(t *testing.T) {
				require.Eventually(t, func() bool {
					err := providerKubeClient.CoreV1().Secrets(upstreamNS).Delete(ctx, "test-secret", metav1.DeleteOptions{})
					return err == nil
				}, wait.ForeverTestTimeout, time.Millisecond*100, "waiting for secret to be deleted at provider")

				require.Eventually(t, func() bool {
					_, err := consumerKubeClient.CoreV1().Secrets(downstreamNS).Get(ctx, "test-secret", metav1.GetOptions{})
					return errors.IsNotFound(err)
				}, wait.ForeverTestTimeout, time.Millisecond*100, "waiting for secret to be deleted at consumer")
			},
		},
	} {
		tc.step(t)
	}
}

func TestConsumerOwned(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	t.Logf("Creating provider workspace")
	providerConfig, providerKubeconfig := framework.NewWorkspace(t, framework.ClientConfig(t), framework.WithGenerateName("test-claimed-resources-consumer"))

	t.Logf("Starting backend with random port")
	addr, _ := framework.StartBackend(t, providerConfig, "--kubeconfig="+providerKubeconfig, "--listen-port=0", "--consumer-scope="+string(kubebindv1alpha1.NamespacedScope))

	t.Logf("Creating MangoDB CRD on provider side")
	consumerfixtures.Bootstrap(t, framework.DiscoveryClient(t, providerConfig), framework.DynamicClient(t, providerConfig), nil)

	t.Logf("Creating consumer workspace and starting konnector")
	consumerConfig, consumerKubeconfig := framework.NewWorkspace(t, framework.ClientConfig(t), framework.WithGenerateName("test-claimed-resources-consumer"))
	framework.StartKonnector(t, consumerConfig, "--kubeconfig="+consumerKubeconfig)

	providerKubeClient := framework.KubeClient(t, providerConfig)
	consumerKubeClient := framework.KubeClient(t, consumerConfig)

	consumerClient := framework.DynamicClient(t, consumerConfig).Resource(
		schema.GroupVersionResource{Group: "mangodb.com", Version: "v1alpha1", Resource: "mangodbs"},
	).Namespace("default")
	providerClient := framework.DynamicClient(t, providerConfig).Resource(
		schema.GroupVersionResource{Group: "mangodb.com", Version: "v1alpha1", Resource: "mangodbs"},
	)

	providerNS := "unknown"
	consumerNS := "unknown"

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
				consumerNS = consumerMangos.Items[0].GetNamespace()

				t.Logf("Waiting for the MangoDB instance to be created on provider side")
				var mangos *unstructured.UnstructuredList
				require.Eventually(t, func() bool {
					var err error
					mangos, err = providerClient.List(ctx, metav1.ListOptions{})
					return err == nil && len(mangos.Items) == 1
				}, wait.ForeverTestTimeout, time.Millisecond*100, "waiting for the MangoDB instance to be created on provider side")

				// this is used everywhere further down
				providerNS = mangos.Items[0].GetNamespace()
			},
		},
		{
			name: "secret creation at consumer",
			step: func(t *testing.T) {
				testSecret := corev1.Secret{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Secret",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-secret",
						Namespace: consumerNS,
					},
					Data: map[string][]byte{
						"test": []byte("dummy"),
					},
				}

				_, err := consumerKubeClient.CoreV1().Secrets(consumerNS).Create(ctx, &testSecret, metav1.CreateOptions{})
				require.NoError(t, err)
			},
		},
		{
			name: "secret is automatically created at provider",
			step: func(t *testing.T) {
				require.Eventually(t, func() bool {
					_, err := providerKubeClient.CoreV1().Secrets(providerNS).Get(ctx, "test-secret", metav1.GetOptions{})
					return err == nil
				}, wait.ForeverTestTimeout, time.Millisecond*100, "waiting for secret to be created on provider side")
			},
		},
		{
			name: "secret updated at provider is overwritten",
			step: func(t *testing.T) {
				require.Eventually(t, func() bool {
					s, err := providerKubeClient.CoreV1().Secrets(providerNS).Get(ctx, "test-secret", metav1.GetOptions{})
					if err != nil {
						return false
					}

					s.Data["test"] = []byte("updated")

					_, err = providerKubeClient.CoreV1().Secrets(providerNS).Update(ctx, s, metav1.UpdateOptions{})
					return err == nil
				}, wait.ForeverTestTimeout, time.Millisecond*100, "waiting for secret to be updated at provider")

				require.Eventually(t, func() bool {
					s, err := providerKubeClient.CoreV1().Secrets(providerNS).Get(ctx, "test-secret", metav1.GetOptions{})
					if err != nil {
						return false
					}

					if v, ok := s.Data["test"]; ok && equality.Semantic.DeepEqual(v, []byte("dummy")) {
						return true
					}
					return false
				}, wait.ForeverTestTimeout, time.Millisecond*100, "waiting for secret to be overwritten at consumer")
			},
		},
		{
			name: "secret is automatically updated at provider",
			step: func(t *testing.T) {
				require.Eventually(t, func() bool {
					s, err := consumerKubeClient.CoreV1().Secrets(consumerNS).Get(ctx, "test-secret", metav1.GetOptions{})
					if err != nil {
						return false
					}

					s.Data["test"] = []byte("updated")

					_, err = consumerKubeClient.CoreV1().Secrets(consumerNS).Update(ctx, s, metav1.UpdateOptions{})
					return err == nil
				}, wait.ForeverTestTimeout, time.Millisecond*100, "waiting for secret to be updated at consumer")

				require.Eventually(t, func() bool {
					s, err := providerKubeClient.CoreV1().Secrets(providerNS).Get(ctx, "test-secret", metav1.GetOptions{})
					if err != nil {
						return false
					}

					if v, ok := s.Data["test"]; ok && equality.Semantic.DeepEqual(v, []byte("updated")) {
						return true
					}
					return false
				}, wait.ForeverTestTimeout, time.Millisecond*100, "waiting for secret to be updated at provider")

			},
		},
		{
			name: "secret deleted at provider is automatically recreated",
			step: func(t *testing.T) {
				require.Eventually(t, func() bool {
					err := providerKubeClient.CoreV1().Secrets(providerNS).Delete(ctx, "test-secret", metav1.DeleteOptions{})
					return err == nil
				}, wait.ForeverTestTimeout, time.Millisecond*100, "waiting for secret to be deleted at provider")

				require.Eventually(t, func() bool {
					_, err := providerKubeClient.CoreV1().Secrets(providerNS).Get(ctx, "test-secret", metav1.GetOptions{})
					return !errors.IsNotFound(err)
				}, wait.ForeverTestTimeout, time.Millisecond*100, "waiting for secret to be recreated at provider")
			},
		},
		{
			name: "secret is automatically deleted at provider",
			step: func(t *testing.T) {
				require.Eventually(t, func() bool {
					err := consumerKubeClient.CoreV1().Secrets(consumerNS).Delete(ctx, "test-secret", metav1.DeleteOptions{})
					return err == nil
				}, wait.ForeverTestTimeout, time.Millisecond*100, "waiting for secret to be deleted at consumer")

				require.Eventually(t, func() bool {
					s, err := providerKubeClient.CoreV1().Secrets(providerNS).Get(ctx, "test-secret", metav1.GetOptions{})
					if errors.IsNotFound(err) {
						return true
					} else {
						t.Logf("secret still exists: %+v", s)
						return false
					}
				}, wait.ForeverTestTimeout, time.Millisecond*100, "waiting for secret to be deleted at provider")
			},
		},
	} {
		tc.step(t)
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

func toUnstructured(t *testing.T, manifest string) *unstructured.Unstructured {
	t.Helper()

	obj := map[string]interface{}{}
	err := yaml.Unmarshal([]byte(manifest), &obj)
	require.NoError(t, err)

	return &unstructured.Unstructured{Object: obj}
}
