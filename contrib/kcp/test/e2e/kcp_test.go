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

package e2e

import (
	"fmt"
	"strings"
	"testing"
	"time"

	kcpclientset "github.com/kcp-dev/kcp/sdk/client/clientset/versioned/cluster"
	kcptesting "github.com/kcp-dev/kcp/sdk/testing"
	kcptestinghelpers "github.com/kcp-dev/kcp/sdk/testing/helpers"
	"github.com/kcp-dev/logicalcluster/v3"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"

	kubebindv1alpha2 "github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2"
	"github.com/kube-bind/kube-bind/test/e2e/framework"
)

func TestKCPClusterScope(t *testing.T) {
	t.Parallel()
	testKcpIntegration(t, kubebindv1alpha2.ClusterScope)
}

func TestKCPNamespacedScope(t *testing.T) {
	t.Parallel()
	testKcpIntegration(t, kubebindv1alpha2.NamespacedScope)
}

func testKcpIntegration(t *testing.T, scope kubebindv1alpha2.InformerScope) {
	t.Helper()
	t.Logf("Testing KCP integration with informer scope %s, tempdir: %s", scope, t.TempDir())

	// dex
	framework.StartDex(t)

	// kcp bootstrap
	server := kcptesting.PrivateKcpServer(t)
	bootstrapKCP(t, server)

	// consumer
	t.Log("Create consumer workspace")
	consumerWsPath, _ := kcptesting.NewWorkspaceFixture(t, server, logicalcluster.NewPath("root"), kcptesting.WithName("consumer"))

	t.Log("Create a consumer kubeconfig")
	consumerCfg, consumerKubeconfig := wsConfig(t, server, consumerWsPath)

	t.Log("Start konnector for consumer workspace")
	framework.StartKonnector(t, consumerCfg, "--kubeconfig="+consumerKubeconfig)

	// backend
	backendAddr := bootstrapBackend(t, server, scope)

	// provider
	t.Log("Create provider workspace")
	providerWsPath, _ := kcptesting.NewWorkspaceFixture(t, server, logicalcluster.NewPath("root"), kcptesting.WithName("provider"))

	t.Log("Create a provider kubeconfig")
	providerCfg, _ := wsConfig(t, server, providerWsPath)

	cfg := server.BaseConfig(t)
	kcpClusterClient, err := kcpclientset.NewForConfig(cfg)
	require.NoError(t, err, "failed to create kcp client")

	t.Log("Bind kube-bind.io APIExport to provider workspace")
	createApiBinding(t,
		kcpClusterClient,
		providerWsPath,
		generateApiBinding(t, logicalcluster.NewPath("root").Join("kube-bind"), "kube-bind.io", "kube-bind.io",
			"clusterrolebindings.rbac.authorization.k8s.io",
			"clusterroles.rbac.authorization.k8s.io",
			"customresourcedefinitions.apiextensions.k8s.io",
			"serviceaccounts.core",
			"configmaps.core",
			"secrets.core",
			"namespaces.core",
			"roles.rbac.authorization.k8s.io",
			"rolebindings.rbac.authorization.k8s.io",
			"apiresourceschemas.apis.kcp.io",
		),
	)

	t.Log("Applying example APIExport, APIResourceSchemas and templates to provider workspace")
	framework.ApplyFiles(t,
		providerCfg,
		"../../deploy/examples/apiexport.yaml",
		"../../deploy/examples/apiresourceschema-cowboys.yaml",  // namespaced
		"../../deploy/examples/apiresourceschema-sheriffs.yaml", // cluster scoped
		"../../deploy/examples/template-cowboys.yaml",           // template for cowboys
		"../../deploy/examples/template-sheriffs.yaml",          // template for sheriffs
		"../../deploy/examples/collection-wildwest.yaml",
	)

	t.Log("Bind the APIExport locally")
	createApiBinding(t,
		kcpClusterClient,
		providerWsPath,
		generateApiBinding(t, providerWsPath, "cowboys-stable", "cowboys-stable"),
	)

	t.Log("Get logical cluster of provider workspace")
	providerCluster, err := kcpClusterClient.Cluster(providerWsPath).CoreV1alpha1().LogicalClusters().Get(t.Context(), "cluster", metav1.GetOptions{})
	require.NoError(t, err)
	require.NotEmpty(t, providerCluster.Status.URL, "provider cluster URL is empty")

	// .Status.URL should have the form: <scheme>://(....)/clusters/<cluster-id>, the <cluster-id> is what is needed for kube-bind
	// The URL overall is irrelevant, so split on `/` and validate that
	// at least the tail end is /cluster/<cluster-id as expected
	providerClusterSplit := strings.Split(providerCluster.Status.URL, "/")
	// Validate the length of the split to prevent panics, need at least
	// two segments, should be something like
	// ["https:", "", "....", "clusters", "<cluster-id>"]
	require.GreaterOrEqual(t, len(providerClusterSplit), 2, "Unexpected URL format: %s", providerCluster.Status.URL)
	// Validate the second last segment is "clusters" as a sanity check
	require.Equal(t, "clusters", providerClusterSplit[len(providerClusterSplit)-2], "Unexpected URL format: %s", providerCluster.Status.URL)
	// Can assume that the last entry is now the cluster-id, grab it and
	// sanity check that it's not empty
	providerClusterID := providerClusterSplit[len(providerClusterSplit)-1]
	require.NotEmpty(t, providerClusterID, "Retreived cluster id is empty, source URL: %s", providerCluster.Status.URL)

	// kube-bind process
	t.Log("Perform binding process with browser")
	var kind, resource, template string
	switch scope {
	case kubebindv1alpha2.ClusterScope:
		kind = "Sheriff"
		resource = "sheriffs"
		template = "sheriffs"
	case kubebindv1alpha2.NamespacedScope:
		kind = "Cowboy"
		resource = "cowboys"
		template = "cowboys"
	default:
		require.Fail(t, "unhandled scope %q", scope)
	}
	performBindingWithBrowser(t, backendAddr, providerClusterID, consumerCfg, consumerKubeconfig, resource, template)

	t.Log("Testing resource creation and synchronization...")
	testKCPResourceSync(t, consumerCfg, providerCfg, scope, kind, resource)
}

func testKcpClient(t testing.TB, cfg *rest.Config, scope kubebindv1alpha2.InformerScope, gvr schema.GroupVersionResource, namespace string) dynamic.ResourceInterface {
	t.Helper()

	client := framework.DynamicClient(t, cfg).Resource(gvr)
	if scope == kubebindv1alpha2.NamespacedScope && namespace != "" {
		return client.Namespace(namespace)
	}
	return client
}

func testKCPResourceSync(t *testing.T, consumerCfg, providerCfg *rest.Config, scope kubebindv1alpha2.InformerScope, kind, resource string) {
	serviceGVR := schema.GroupVersionResource{Group: "wildwest.dev", Version: "v1alpha1", Resource: resource}

	consumerClient := testKcpClient(t, consumerCfg, scope, serviceGVR, "default")
	// provider side has a namespace with a random name for the consumer
	// so it needs to be a non namespaced client
	providerClient := testKcpClient(t, providerCfg, scope, serviceGVR, "")

	t.Run("instance created downstream syncs upstream", func(t *testing.T) {
		t.Logf("Creating %s instance on consumer side", kind)

		resourceInstance := &unstructured.Unstructured{
			Object: map[string]any{
				"apiVersion": "wildwest.dev/v1alpha1",
				"kind":       kind,
				"metadata": map[string]any{
					"name": "test-" + resource,
				},
				"spec": map[string]any{
					"intent": "draw",
				},
			},
		}

		kcptestinghelpers.Eventually(t, func() (bool, string) {
			_, err := consumerClient.Create(t.Context(), resourceInstance, metav1.CreateOptions{})
			return err == nil, fmt.Sprintf("Error creating %s instance: %v", resource, err)
		}, wait.ForeverTestTimeout, time.Millisecond*100)

		t.Logf("Waiting for %s instance to be synced to provider side", resource)
		var instances *unstructured.UnstructuredList
		kcptestinghelpers.Eventually(t, func() (bool, string) {
			var err error
			instances, err = providerClient.List(t.Context(), metav1.ListOptions{})
			return err == nil && len(instances.Items) >= 1, fmt.Sprintf("Error listing %s instances: %v", resource, err)
		}, wait.ForeverTestTimeout, time.Millisecond*100)

		require.Equal(t, 1, len(instances.Items), "Expected exactly one %s instance on provider side", resource)
	})

	t.Run("instance spec updated downstream syncs upstream", func(t *testing.T) {
		t.Logf("Updating %s spec on consumer side", resource)

		require.Eventually(t, func() bool {
			obj, err := consumerClient.Get(t.Context(), "test-"+resource, metav1.GetOptions{})
			if err != nil {
				return false
			}

			unstructured.SetNestedField(obj.Object, "holster", "spec", "intent") //nolint:errcheck
			_, err = consumerClient.Update(t.Context(), obj, metav1.UpdateOptions{})
			return err == nil
		}, wait.ForeverTestTimeout, 5*time.Second, "waiting for %s spec to be updated on consumer side", resource)

		t.Logf("Waiting for %s spec update to sync to provider side", resource)
		require.Eventually(t, func() bool {
			instances, err := providerClient.List(t.Context(), metav1.ListOptions{})
			if err != nil || len(instances.Items) == 0 {
				return false
			}

			intent, found, err := unstructured.NestedString(instances.Items[0].Object, "spec", "intent")
			return err == nil && found && intent == "holster"
		}, wait.ForeverTestTimeout, 5*time.Second, "waiting for %s spec update to sync to provider side", resource)
	})

	t.Run("instance deleted downstream is deleted upstream", func(t *testing.T) {
		t.Logf("Deleting %s instance on consumer side", resource)

		err := consumerClient.Delete(t.Context(), "test-"+resource, metav1.DeleteOptions{})
		require.NoError(t, err, "Failed to delete %s on consumer side", resource)

		t.Logf("Waiting for %s instance to be deleted on provider side", resource)
		require.Eventually(t, func() bool {
			instances, err := providerClient.List(t.Context(), metav1.ListOptions{})
			return err == nil && len(instances.Items) == 0
		}, wait.ForeverTestTimeout, 5*time.Second, "waiting for %s instance to be deleted on provider side", resource)
	})
}
