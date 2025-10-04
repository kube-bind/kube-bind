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
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/securecookie"
	"github.com/headzoo/surf"
	kcpapisv1alpha2 "github.com/kcp-dev/kcp/sdk/apis/apis/v1alpha2"
	kcpclientset "github.com/kcp-dev/kcp/sdk/client/clientset/versioned/cluster"
	kcptestinghelpers "github.com/kcp-dev/kcp/sdk/testing/helpers"
	kcptestingserver "github.com/kcp-dev/kcp/sdk/testing/server"
	"github.com/kcp-dev/logicalcluster/v3"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/yaml"

	backend "github.com/kube-bind/kube-bind/backend"
	"github.com/kube-bind/kube-bind/contrib/kcp/bootstrap"
	"github.com/kube-bind/kube-bind/contrib/kcp/bootstrap/options"
	"github.com/kube-bind/kube-bind/test/e2e/framework"
)

func init() {
	// avoid warnings from controller-runtime about a missing logger
	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))
	log.SetLogger(klog.NewKlogr())
}

func generateApiBinding(t testing.TB, path logicalcluster.Path, name, exportName string, acceptPermissionClaims ...string) *kcpapisv1alpha2.APIBinding {
	t.Helper()

	apiBinding := &kcpapisv1alpha2.APIBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: kcpapisv1alpha2.APIBindingSpec{
			Reference: kcpapisv1alpha2.BindingReference{
				Export: &kcpapisv1alpha2.ExportBindingReference{
					Path: path.String(),
					Name: exportName,
				},
			},
		},
	}

	for _, claim := range acceptPermissionClaims {
		split := strings.SplitN(claim, ".", 2)
		require.Len(t, split, 2, "invalid permission claim: %s, must be in the form <resource>.<group>", claim)

		resource, group := split[0], split[1]
		require.NotEmpty(t, group, "invalid permission claim: %s, group cannot be empty", claim)
		require.NotEmpty(t, resource, "invalid permission claim: %s, resource cannot be empty", claim)

		if group == "core" {
			group = ""
		}

		apiBinding.Spec.PermissionClaims = append(
			apiBinding.Spec.PermissionClaims,
			kcpapisv1alpha2.AcceptablePermissionClaim{
				State: kcpapisv1alpha2.ClaimAccepted,
				ScopedPermissionClaim: kcpapisv1alpha2.ScopedPermissionClaim{
					Selector: kcpapisv1alpha2.PermissionClaimSelector{
						MatchAll: true,
					},
					PermissionClaim: kcpapisv1alpha2.PermissionClaim{
						GroupResource: kcpapisv1alpha2.GroupResource{
							Group:    group,
							Resource: resource,
						},
						Verbs: []string{"*"},
					},
				},
			},
		)
	}

	return apiBinding
}

func createApiBinding(t testing.TB, client *kcpclientset.ClusterClientset, path logicalcluster.Path, apiBinding *kcpapisv1alpha2.APIBinding) *kcpapisv1alpha2.APIBinding {
	t.Helper()

	_, err := client.Cluster(path).
		ApisV1alpha2().
		APIBindings().
		Create(t.Context(), apiBinding, metav1.CreateOptions{})
	require.NoError(t, err, "Error creating APIBinding")

	var inCluster *kcpapisv1alpha2.APIBinding
	kcptestinghelpers.Eventually(t, func() (bool, string) {
		var err error
		inCluster, err = client.
			Cluster(path).
			ApisV1alpha2().
			APIBindings().
			Get(t.Context(), apiBinding.Name, metav1.GetOptions{})
		if err != nil {
			return false, fmt.Sprintf("Error getting APIBinding: %v", err)
		}
		return inCluster.Status.Phase == kcpapisv1alpha2.APIBindingPhaseBound, fmt.Sprintf("APIBinding not ready: %#v", inCluster.Status)
	}, wait.ForeverTestTimeout, time.Millisecond*100)
	return inCluster
}

func bootstrapKCP(t testing.TB, server kcptestingserver.RunningServer) {
	t.Helper()
	t.Log("Bootstrapping KCP")

	cfg := server.BaseConfig(t)
	cfg.Host += "/clusters/root"
	adminApiCfg := framework.RestToKubeconfig(cfg, "default")
	adminKubeconfig := framework.WriteKubeconfig(t, adminApiCfg, "admin.kubeconfig")

	fs := pflag.NewFlagSet("kcp-bootstrap", pflag.ContinueOnError)
	options := options.NewOptions()
	options.AddFlags(fs)

	options.KCPKubeConfig = adminKubeconfig

	completed, err := options.Complete()
	require.NoError(t, err)

	config, err := bootstrap.NewConfig(completed)
	require.NoError(t, err)

	bootstrapper, err := bootstrap.NewServer(t.Context(), config)
	require.NoError(t, err)
	require.NoError(t, bootstrapper.Start(t.Context()))
}

// startBackend is a copy of framework.StartBackend but skips the CRDs
// (which clashes with the APIResourceSchemas installed by kcp-init.
func startBackend(t *testing.T, args ...string) (string, *backend.Server) {
	signingKey := securecookie.GenerateRandomKey(32)
	require.NotEmpty(t, signingKey, "error creating signing key")
	encryptionKey := securecookie.GenerateRandomKey(32)
	require.NotEmpty(t, encryptionKey, "error creating encryption key")

	addr := "127.0.0.1:8080"

	args = append(
		[]string{
			"--oidc-issuer-client-secret=ZXhhbXBsZS1hcHAtc2VjcmV0",
			"--oidc-issuer-client-id=kube-bind",
			"--oidc-issuer-url=http://127.0.0.1:5556/dex",
			"--cookie-signing-key=" + base64.StdEncoding.EncodeToString(signingKey),
			"--cookie-encryption-key=" + base64.StdEncoding.EncodeToString(encryptionKey),

			"--listen-address=" + addr,
			"--oidc-callback-url=http://127.0.0.1:8080/callback",
		},
		args...,
	)

	backendCmd := exec.CommandContext(t.Context(),
		"../../../../bin/backend",
		args...,
	)
	backendCmd.Stdout = newLogWriter("[backend stdout] ", t)
	backendCmd.Stderr = newLogWriter("[backend stderr] ", t)
	require.NoError(t, backendCmd.Start())
	t.Cleanup(func() {
		if backendCmd.Process != nil {
			t.Logf("Stopping dex (PID: %d)", backendCmd.Process.Pid)
			assert.NoError(t, backendCmd.Process.Kill())
		}
	})
	return addr, nil

	// fs := pflag.NewFlagSet("backend", pflag.ContinueOnError)
	// opts := options.NewOptions()
	// opts.AddFlags(fs)
	// err := fs.Parse(args)
	// require.NoError(t, err)
	//
	// t.Logf("starting backend with options: %#v", opts)
	//
	// // use a random port via an explicit listener. Then add a kube-bind-<port> client to dex
	// // with the callback URL set to the listener's address.
	// opts.Serve.Listener, err = net.Listen("tcp", "localhost:0")
	// require.NoError(t, err)
	// addr := opts.Serve.Listener.Addr()
	// _, port, err := net.SplitHostPort(addr.String())
	// require.NoError(t, err)
	//
	// opts.OIDC.IssuerClientID = "kube-bind-" + port
	// framework.CreateDexClient(t, addr)
	//
	// opts.ExtraOptions.TestingSkipNameValidation = true
	// opts.ExtraOptions.SchemaSource = options.CustomResourceDefinitionSource.String()
	//
	// completed, err := opts.Complete()
	// require.NoError(t, err)
	//
	// config, err := backend.NewConfig(completed)
	// require.NoError(t, err)
	//
	// server, err := backend.NewServer(t.Context(), config)
	// require.NoError(t, err)
	//
	// err = server.Run(t.Context())
	// require.NoError(t, err)
	// t.Logf("backend listening on %s", addr)
	//
	// return addr, server
}

func bootstrapBackend(t *testing.T, server kcptestingserver.RunningServer) string {
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
	addr, _ := startBackend(t,
		"--kubeconfig="+backendKubeconfig,
		"--multicluster-runtime-provider=kcp",
		"--server-url="+exportUrl,
		"--pretty-name=BigCorp.com",
		"--namespace-prefix=kube-bind-",
		"--schema-source=apiresourceschemas",
		"--consumer-scope=cluster", // TODO configure to test both modes
	)

	t.Log("Wait for backend to be ready")
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://"+addr+"/healthz", nil)
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

	return addr
}

func wsConfig(t testing.TB, server kcptestingserver.RunningServer, workspace logicalcluster.Path) (*rest.Config, string) {
	cfg := server.BaseConfig(t)
	cfg.Host += "/clusters/" + workspace.String()

	kubeconfig := framework.WriteKubeconfig(t,
		framework.RestToKubeconfig(cfg, "default"),
		workspace.Base()+".kubeconfig",
	)

	return cfg, kubeconfig
}

func applyFile(t testing.TB, kubeconfig, file string) {
	t.Helper()
	t.Logf("Applying file %q", file)

	// TODO: built a dynamic client to apply the files instead of using
	// kubectl

	cmd := exec.CommandContext(t.Context(), "kubectl", "apply", "-f", file)
	cmd.Env = append(os.Environ(), "KUBECONFIG="+kubeconfig)
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "failed to apply file %q: %s", file, output)
}

func performBindingWithBrowser(t *testing.T, backendAddr string, clusterID string, consumerCfg *rest.Config, consumerKubeconfig string) {
	bindURL := fmt.Sprintf("http://%s/clusters/%s/exports", backendAddr, clusterID)
	t.Logf("Bind URL: %s", bindURL)

	// Test binding dry run first (similar to happy-case test)
	t.Run("Service is bound dry run", func(t *testing.T) {
		authURLDryRunCh := make(chan string, 1)
		go simulateKCPBrowser(t, authURLDryRunCh, "cowboys")

		iostreams, _, bufOut, _ := genericclioptions.NewTestIOStreams()
		framework.Bind(t, iostreams, authURLDryRunCh, nil, bindURL, "--kubeconfig", consumerKubeconfig, "--dry-run")
		_, err := yaml.YAMLToJSON(bufOut.Bytes())
		require.NoError(t, err, "Generated output is not valid YAML")
	})

	// Perform actual binding (similar to happy-case test)
	t.Run("Service is bound", func(t *testing.T) {
		authURLCh := make(chan string, 1)
		go simulateKCPBrowser(t, authURLCh, "cowboys")

		iostreams, _, _, _ := genericclioptions.NewTestIOStreams()
		invocations := make(chan framework.SubCommandInvocation, 1)
		framework.Bind(t, iostreams, authURLCh, invocations, bindURL, "--kubeconfig", consumerKubeconfig)
		inv := <-invocations

		inv.Args = append(
			inv.Args,
			"--kubeconfig="+consumerKubeconfig,
			"--skip-konnector=true",
			"--no-banner",
			"-f=-", // api service export from stdin
		)

		framework.BindAPIService(t, inv.Stdin, "", inv.Args...)

		// Wait for CRD to be created on consumer side
		t.Logf("Waiting for cowboy CRD to be created on consumer side")
		crdClient := framework.ApiextensionsClient(t, consumerCfg).ApiextensionsV1().CustomResourceDefinitions()
		require.Eventually(t, func() bool {
			_, err := crdClient.Get(context.Background(), "cowboys.wildwest.dev", metav1.GetOptions{})
			return err == nil
		}, wait.ForeverTestTimeout, time.Millisecond*100)
	})
}

// simulateKCPBrowser simulates browser interaction for KCP binding.
func simulateKCPBrowser(t *testing.T, authURLCh chan string, resource string) {
	browser := surf.NewBrowser()
	authURL := <-authURLCh

	t.Logf("Browsing to auth URL: %s", authURL)
	err := browser.Open(authURL)
	require.NoError(t, err, "Failed to open auth URL")

	t.Logf("Waiting for browser to be at /resources")
	framework.BrowserEventuallyAtPath(t, browser, "/resources")

	t.Logf("Clicking %s resource", resource)
	err = browser.Click("a." + resource)
	require.NoError(t, err, "Failed to click resource link")

	t.Logf("Waiting for browser to be forwarded to client")
	framework.BrowserEventuallyAtPath(t, browser, "/callback")
}

// toUnstructured converts YAML manifest to unstructured object (from happy-case test).
func toUnstructured(t *testing.T, manifest string) *unstructured.Unstructured {
	t.Helper()

	obj := map[string]any{}
	err := yaml.Unmarshal([]byte(manifest), &obj)
	require.NoError(t, err, "Failed to unmarshal YAML manifest")

	return &unstructured.Unstructured{Object: obj}
}
