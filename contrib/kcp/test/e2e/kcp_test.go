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
	"k8s.io/client-go/rest"

	"github.com/kube-bind/kube-bind/test/e2e/framework"
)

// TODO(ntnn): Better name pattern.
func TestKCPIntegration(t *testing.T) {
	t.Logf("tempdir: %s", t.TempDir())

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
	backendAddr := bootstrapBackend(t, server)

	// provider
	t.Log("Create provider workspace")
	providerWsPath, _ := kcptesting.NewWorkspaceFixture(t, server, logicalcluster.NewPath("root"), kcptesting.WithName("provider"))

	t.Log("Create a provider kubeconfig")
	providerCfg, providerKubeconfig := wsConfig(t, server, providerWsPath)

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

	t.Log("Applying example APIExport and APIResourceSchemas to provider workspace")
	applyFile(t, providerKubeconfig, "../../deploy/examples/apiexport.yaml")
	applyFile(t, providerKubeconfig, "../../deploy/examples/apiresourceschema-cowboys.yaml")
	applyFile(t, providerKubeconfig, "../../deploy/examples/apiresourceschema-sheriffs.yaml")

	t.Log("Bind the APIExport locally")
	createApiBinding(t,
		kcpClusterClient,
		providerWsPath,
		generateApiBinding(t, providerWsPath, "cowboys-stable", "cowboys-stable"),
	)

	t.Log("Get logical cluster of provider workspace")
	providerCluster, err := kcpClusterClient.Cluster(providerWsPath).CoreV1alpha1().LogicalClusters().Get(t.Context(), "cluster", metav1.GetOptions{})
	require.NoError(t, err)

	providerClusterSplit := strings.Split(providerCluster.Status.URL, "/")
	require.GreaterOrEqual(t, len(providerClusterSplit), 2, "Unexpected URL format: %s", providerCluster.Status.URL)
	require.Equal(t, "clusters", providerClusterSplit[len(providerClusterSplit)-2], "Unexpected URL format: %s", providerCluster.Status.URL)
	providerClusterID := providerClusterSplit[len(providerClusterSplit)-1]
	require.NotEmpty(t, providerClusterID, "Unexpected URL format: %s", providerCluster.Status.URL)

	// kube-bind process
	t.Log("Perform binding process with browser")
	performBindingWithBrowser(t, backendAddr, providerClusterID, consumerCfg, consumerKubeconfig)

	t.Log("Testing resource creation and synchronization...")
	testKCPResourceSync(t, consumerCfg, providerCfg)

	t.Log("Success")
}

func testKCPResourceSync(t *testing.T, consumerCfg, providerCfg *rest.Config) {
	serviceGVR := schema.GroupVersionResource{Group: "wildwest.dev", Version: "v1alpha1", Resource: "cowboys"}

	consumerClient := framework.DynamicClient(t, consumerCfg).Resource(serviceGVR)
	providerClient := framework.DynamicClient(t, providerCfg).Resource(serviceGVR)

	cowboyInstance := `
apiVersion: wildwest.dev/v1alpha1
kind: Cowboy
metadata:
  name: test-cowboy
spec:
  intent: "draw"
`

	for _, tc := range []struct {
		name string
		step func(t *testing.T)
	}{
		{
			name: "instance created downstream syncs upstream",
			step: func(t *testing.T) {
				t.Logf("Creating cowboy instance on consumer side")

				kcptestinghelpers.Eventually(t, func() (bool, string) {
					_, err := consumerClient.Namespace("default").Create(t.Context(), toUnstructured(t, cowboyInstance), metav1.CreateOptions{})
					return err == nil, fmt.Sprintf("Error creating cowboy instance: %v", err)
				}, wait.ForeverTestTimeout, time.Millisecond*100)

				t.Logf("Waiting for cowboy instance to be synced to provider side")
				var instances *unstructured.UnstructuredList
				kcptestinghelpers.Eventually(t, func() (bool, string) {
					var err error
					instances, err = providerClient.List(t.Context(), metav1.ListOptions{})
					return err == nil && len(instances.Items) >= 1, fmt.Sprintf("Error listing cowboy instances: %v", err)
				}, wait.ForeverTestTimeout, time.Millisecond*100)

				t.Logf("Found %d cowboy instances on provider side", len(instances.Items))
			},
		},
		{
			name: "instance spec updated downstream syncs upstream",
			step: func(t *testing.T) {
				t.Logf("Updating cowboy spec on consumer side")

				require.Eventually(t, func() bool {
					obj, err := consumerClient.Namespace("default").Get(t.Context(), "test-cowboy", metav1.GetOptions{})
					if err != nil {
						return false
					}

					unstructured.SetNestedField(obj.Object, "holster", "spec", "intent") //nolint:errcheck
					_, err = consumerClient.Namespace("default").Update(t.Context(), obj, metav1.UpdateOptions{})
					return err == nil
				}, 2*time.Minute, 5*time.Second, "waiting for cowboy spec to be updated on consumer side")

				t.Logf("Waiting for cowboy spec update to sync to provider side")
				require.Eventually(t, func() bool {
					instances, err := providerClient.List(t.Context(), metav1.ListOptions{})
					if err != nil || len(instances.Items) == 0 {
						return false
					}

					intent, found, err := unstructured.NestedString(instances.Items[0].Object, "spec", "intent")
					return err == nil && found && intent == "holster"
				}, 5*time.Minute, 5*time.Second, "waiting for cowboy spec update to sync to provider side")
			},
		},
		{
			name: "instance deleted downstream is deleted upstream",
			step: func(t *testing.T) {
				t.Logf("Deleting cowboy instance on consumer side")

				err := consumerClient.Namespace("default").Delete(t.Context(), "test-cowboy", metav1.DeleteOptions{})
				require.NoError(t, err, "Failed to delete cowboy on consumer side")

				t.Logf("Waiting for cowboy instance to be deleted on provider side")
				require.Eventually(t, func() bool {
					instances, err := providerClient.List(t.Context(), metav1.ListOptions{})
					return err == nil && len(instances.Items) == 0
				}, 5*time.Minute, 5*time.Second, "waiting for cowboy instance to be deleted on provider side")
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.step(t)
		})
	}

	t.Log("KCP resource synchronization tests passed!")
}
