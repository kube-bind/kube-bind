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

package bind

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/headzoo/surf"
	kcpclientset "github.com/kcp-dev/kcp/sdk/client/clientset/versioned"
	"github.com/kcp-dev/logicalcluster/v3"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/yaml"

	bootstrap "github.com/kube-bind/kube-bind/contrib/example-backend-kcp/bootstrap/config/kube-bind"
	"github.com/kube-bind/kube-bind/contrib/example-backend-kcp/test/e2e/framework"
	kubebindv1alpha1 "github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha1"
	providerfixtures "github.com/kube-bind/kube-bind/test/e2e/bind/fixtures/provider"
)

func TestClusterScoped(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	restConfig := framework.ClientConfig(t)
	ret := rest.CopyConfig(restConfig)

	wsAdminKubeconfigPath := filepath.Join(framework.WorkDir, "admin.kubeconfig")
	err := clientcmd.WriteToFile(framework.RestToKubeconfig(ret, ""), wsAdminKubeconfigPath)
	if err != nil {
		t.Fatalf("Failed to write kubeconfig: %v", err)
	}

	t.Logf("Bootstrapping backend with [%s]", wsAdminKubeconfigPath)
	err = framework.BootstrapBackend(t, "--kubeconfig="+wsAdminKubeconfigPath)
	if err != nil {
		t.Fatalf("Failed to bootstrap backend: %v", err)
	}

	t.Logf("Starting backend with random port")
	addr, _ := framework.StartBackend(t, restConfig, "--kubeconfig="+wsAdminKubeconfigPath, "--listen-port=0", "--consumer-scope="+string(kubebindv1alpha1.ClusterScope))
	t.Logf("Creating provider workspace")
	providerConfig, _ := framework.NewWorkspace(t, framework.ClientConfig(t), framework.WithGenerateName("provider-"))

	t.Logf("Creating CRDs on provider side")
	providerfixtures.Bootstrap(t, framework.DiscoveryClient(t, providerConfig), framework.DynamicClient(t, providerConfig), nil)

	// bind the provider workspace to the backend
	t.Logf("Binding provider workspace to backend")
	kcpClient, err := kcpclientset.NewForConfig(providerConfig)
	if err != nil {
		t.Fatalf("Failed to create kcp client: %v", err)
	}

	provider := logicalcluster.NewPath("root:kube-bind")
	err = bootstrap.BindAPIExport(ctx, kcpClient, "kube-bind.io", provider)

	t.Logf("Creating consumer workspace")
	consumerConfig, _ := framework.NewWorkspace(t, framework.ClientConfig(t), framework.WithGenerateName("consumer-"))

	serviceGVR := schema.GroupVersionResource{Group: "mangodb.com", Version: "v1alpha1", Resource: "mangodbs"}

	mangodbInstance := `
	apiVersion: mangodb.com/v1alpha1
	kind: MangoDB
	metadata:
	  name: test
	spec:
	  tokenSecret: credentials
	`

	t.Logf("Bound service dry run")
	iostreams, _, bufOut, _ := genericclioptions.NewTestIOStreams()
	authURLDryRunCh := make(chan string, 1)
	go simulateBrowser(t, authURLDryRunCh, serviceGVR.Resource)
	framework.Bind(t, iostreams, authURLDryRunCh, nil, fmt.Sprintf("http://%s/export", addr.String()), "--kubeconfig", consumerKubeconfig, "--skip-konnector", "--dry-run")
	_, err := yaml.YAMLToJSON(bufOut.Bytes())
	require.NoError(t, err)

	time.Sleep(5 * time.Minute)
	ctx.Done()
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
