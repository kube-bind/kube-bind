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
	"net/http"
	"testing"
	"time"

	kcpclientset "github.com/kcp-dev/kcp/sdk/client/clientset/versioned/cluster"
	kcptestinghelpers "github.com/kcp-dev/kcp/sdk/testing/helpers"
	kcptestingserver "github.com/kcp-dev/kcp/sdk/testing/server"
	"github.com/kcp-dev/logicalcluster/v3"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	kubebindv1alpha2 "github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2"
	"github.com/kube-bind/kube-bind/test/e2e/framework"
)

func bootstrapBackend(t *testing.T, server kcptestingserver.RunningServer, scope kubebindv1alpha2.InformerScope) string {
	t.Helper()
	t.Log("Bootstrapping backend")

	client, err := kcpclientset.NewForConfig(server.BaseConfig(t))
	require.NoError(t, err)

	exportUrl := ""
	kcptestinghelpers.Eventually(t, func() (bool, string) {
		exportES, err := client.Cluster(logicalcluster.NewPath("root").Join("kube-bind")).
			ApisV1alpha1().
			APIExportEndpointSlices().
			Get(t.Context(), "kube-bind.io", metav1.GetOptions{})
		if err != nil {
			return false, fmt.Sprintf("Error getting APIExportEndpointSlice: %v", err)
		}
		if len(exportES.Status.APIExportEndpoints) == 0 {
			return false, "APIExportEndpoints is empty"
		}
		exportUrl = exportES.Status.APIExportEndpoints[0].URL
		return true, ""
	}, wait.ForeverTestTimeout, time.Millisecond*100)
	require.NotEmpty(t, exportUrl, "APIExportEndpointSlice URL is empty")

	_, backendKubeconfig := wsConfig(t, server, logicalcluster.NewPath("root").Join("kube-bind"))

	t.Log("Starting kube-bind backend for KCP")
	addr, _ := framework.StartBackend(t,
		"--kubeconfig="+backendKubeconfig,
		"--multicluster-runtime-provider=kcp",
		"--server-url="+exportUrl,
		"--pretty-name=BigCorp.com",
		"--namespace-prefix=kube-bind-",
		"--schema-source=apiresourceschemas",
		"--consumer-scope="+string(scope),
	)

	t.Log("Wait for backend to be ready")
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://"+addr.String()+"/healthz", nil)
	require.NoError(t, err)
	kcptestinghelpers.Eventually(t, func() (bool, string) {
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return false, ""
		}
		defer resp.Body.Close()
		return resp.StatusCode == http.StatusOK, ""
	}, wait.ForeverTestTimeout, time.Millisecond*100)
	t.Log("Backend is ready")

	return addr.String()
}
