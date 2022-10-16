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
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gopkg.in/headzoo/surf.v1"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/yaml"

	providerfixtures "github.com/kube-bind/kube-bind/test/e2e/bind/fixtures/provider"
	"github.com/kube-bind/kube-bind/test/e2e/framework"
)

func TestHappyCase(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	t.Logf("Creating provider workspace")
	providerConfig, providerKubeconfig := framework.NewWorkspace(t, framework.ClientConfig(t), framework.WithGenerateName("test-happy-case-provider"))

	t.Logf("Creating MangoDB CRD on provider side")
	providerfixtures.Bootstrap(t, framework.DiscoveryClient(t, providerConfig), framework.DynamicClient(t, providerConfig), nil)

	t.Logf("Starting backend with random port")
	addr, _ := framework.StartBackend(t, providerConfig, "--kubeconfig="+providerKubeconfig, "--listen-port=0")

	t.Logf("Creating consumer workspace and starting konnector")
	consumerConfig, consumerKubeconfig := framework.NewWorkspace(t, framework.ClientConfig(t), framework.WithGenerateName("test-happy-case-consumer"))
	framework.StartKonnector(t, consumerConfig, "--kubeconfig="+consumerKubeconfig)

	consumerClient := framework.DynamicClient(t, consumerConfig).Resource(
		schema.GroupVersionResource{Group: "mangodb.com", Version: "v1alpha1", Resource: "mangodbs"},
	).Namespace("default")
	providerClient := framework.DynamicClient(t, providerConfig).Resource(
		schema.GroupVersionResource{Group: "mangodb.com", Version: "v1alpha1", Resource: "mangodbs"},
	)

	upstreamNS := "unknown"

	for _, tc := range []struct {
		name string
		step func(t *testing.T)
	}{
		{
			name: "MangoDB is bound",
			step: func(t *testing.T) {
				authURLCh := make(chan string, 1)
				go simulateBrowser(t, authURLCh, "mangodbs")
				framework.Bind(t, authURLCh, fmt.Sprintf("http://%s/export", addr.String()), "--kubeconfig", consumerKubeconfig, "--skip-konnector")

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
