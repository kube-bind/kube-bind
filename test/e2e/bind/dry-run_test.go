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
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	bindapiservice "github.com/kube-bind/kube-bind/cli/pkg/kubectl/bind-apiservice/plugin"
	examples "github.com/kube-bind/kube-bind/deploy/examples"
	kubebindv1alpha2 "github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2"
	"github.com/kube-bind/kube-bind/test/e2e/framework"
)

func TestDryRunClusterScoped(t *testing.T) {
	t.Parallel()
	testDryRun(t, "cc", apiextensionsv1.ClusterScoped, kubebindv1alpha2.ClusterScope)
}

func testDryRun(
	t *testing.T,
	name string,
	resourceScope apiextensionsv1.ResourceScope,
	informerScope kubebindv1alpha2.InformerScope,
) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	framework.StartDex(t)

	suffix := framework.RandomString(4)

	t.Logf("Creating provider workspace")
	providerConfig, providerKubeconfig := framework.NewWorkspace(t, framework.ClientConfig(t), framework.WithName("%s-provider-%s", name, suffix))

	t.Logf("Installing kubebind CRDs")
	framework.InstallKubebindCRDs(t, providerConfig)

	t.Logf("Starting backend with random port")
	addr, _ := framework.StartBackend(t, "--kubeconfig="+providerKubeconfig, "--listen-address=:0", "--consumer-scope="+string(informerScope))

	t.Logf("Creating CRD on provider side")
	examples.Bootstrap(t, framework.DiscoveryClient(t, providerConfig), framework.DynamicClient(t, providerConfig), nil)

	t.Logf("Creating consumer workspace and starting konnector")
	consumerConfig, consumerKubeconfig := framework.NewWorkspace(t, framework.ClientConfig(t), framework.WithName("%s-consumer-%s", name, suffix))
	framework.StartKonnector(t, consumerConfig, "--kubeconfig="+consumerKubeconfig)

	serviceGVR := schema.GroupVersionResource{Group: "wildwest.dev", Version: "v1alpha1", Resource: "cowboys"}
	if resourceScope == apiextensionsv1.ClusterScoped {
		serviceGVR = schema.GroupVersionResource{Group: "wildwest.dev", Version: "v1alpha1", Resource: "sheriffs"}
	}
	templateRef := "cowboys"
	if resourceScope == apiextensionsv1.ClusterScoped {
		templateRef = "sheriffs"
	}

	consumerBindClient := framework.BindClient(t, consumerConfig)

	// Create a temporary directory for dry-run assets
	tempHomeDir := t.TempDir()
	originalHomeDir := os.Getenv("HOME")
	os.Setenv("HOME", tempHomeDir)
	t.Cleanup(func() {
		if originalHomeDir != "" {
			os.Setenv("HOME", originalHomeDir)
		} else {
			os.Unsetenv("HOME")
		}
	})

	kubeBindConfig := filepath.Join(framework.WorkDir, fmt.Sprintf("kube-bind-config-%s.yaml", suffix))

	var bindResponse *kubebindv1alpha2.BindingResourceResponse
	var sessionID string
	for _, tc := range []struct {
		name string
		step func(t *testing.T)
	}{
		{
			name: "Login to provider",
			step: func(t *testing.T) {
				iostreams, _, _, _ := genericclioptions.NewTestIOStreams()
				authURLDryRunCh := make(chan string, 1)
				go framework.SimulateBrowser(t, authURLDryRunCh)
				framework.Login(t, iostreams, authURLDryRunCh, kubeBindConfig, fmt.Sprintf("http://%s/api/exports", addr.String()), "")
			},
		},
		{
			name: "Get bind APIServiceExportRequest from server",
			step: func(t *testing.T) {
				c := framework.GetKubeBindRestClient(t, kubeBindConfig)
				var err error
				bindResponse, err = c.Bind(ctx, &kubebindv1alpha2.BindableResourcesRequest{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-binding",
					},
					TemplateRef: kubebindv1alpha2.APIServiceExportTemplateRef{
						Name: templateRef,
					},
				})
				require.NoError(t, err)
				require.NotNil(t, bindResponse)
				require.NotNil(t, bindResponse.Authentication.OAuth2CodeGrant)
				sessionID = bindResponse.Authentication.OAuth2CodeGrant.SessionID
				require.NotEmpty(t, sessionID)
			},
		},
		{
			name: "Run dry-run with template - verify no consumer cluster resources created",
			step: func(t *testing.T) {
				iostreams, _, _, _ := genericclioptions.NewTestIOStreams()
				opts := bindapiservice.NewBindAPIServiceOptions(iostreams)
				opts.ConfigFile = kubeBindConfig
				opts.Kubeconfig = consumerKubeconfig
				opts.Template = templateRef
				opts.DryRun = true
				opts.SkipKonnector = true

				err := opts.Complete(nil)
				require.NoError(t, err)
				err = opts.Validate()
				require.NoError(t, err)

				bindings, err := consumerBindClient.KubeBindV1alpha2().APIServiceBindings().List(ctx, metav1.ListOptions{})
				require.NoError(t, err)
				initialBindingCount := len(bindings.Items)

				crdClient := framework.ApiextensionsClient(t, consumerConfig).ApiextensionsV1().CustomResourceDefinitions()
				_, err = crdClient.Get(ctx, serviceGVR.Resource+"."+serviceGVR.Group, metav1.GetOptions{})
				require.True(t, errors.IsNotFound(err), "CRD should not exist before dry-run")

				err = opts.Run(ctx)
				require.NoError(t, err)

				bindings, err = consumerBindClient.KubeBindV1alpha2().APIServiceBindings().List(ctx, metav1.ListOptions{})
				require.NoError(t, err)
				require.Equal(t, initialBindingCount, len(bindings.Items), "No bindings should be created during dry-run")

				_, err = crdClient.Get(ctx, serviceGVR.Resource+"."+serviceGVR.Group, metav1.GetOptions{})
				require.True(t, errors.IsNotFound(err), "CRD should not be created during dry-run")
			},
		},
		{
			name: "Apply from dry-run assets - verify bindings are created",
			step: func(t *testing.T) {
				iostreams, _, _, _ := genericclioptions.NewTestIOStreams()
				opts := bindapiservice.NewBindAPIServiceOptions(iostreams)
				opts.ConfigFile = kubeBindConfig
				opts.Kubeconfig = consumerKubeconfig
				opts.FromDryRun = sessionID
				opts.SkipKonnector = true

				err := opts.Complete(nil)
				require.NoError(t, err)
				err = opts.Validate()
				require.NoError(t, err)

				err = opts.Run(ctx)
				require.NoError(t, err)

				t.Logf("Waiting for %s CRD to be created on consumer side", serviceGVR.Resource)
				crdClient := framework.ApiextensionsClient(t, consumerConfig).ApiextensionsV1().CustomResourceDefinitions()
				require.Eventually(t, func() bool {
					_, err := crdClient.Get(ctx, serviceGVR.Resource+"."+serviceGVR.Group, metav1.GetOptions{})
					return err == nil
				}, wait.ForeverTestTimeout, time.Millisecond*100, "waiting for %s CRD to be created on consumer side", serviceGVR.Resource)

				bindings, err := consumerBindClient.KubeBindV1alpha2().APIServiceBindings().List(ctx, metav1.ListOptions{})
				require.NoError(t, err)
				require.Greater(t, len(bindings.Items), 0, "At least one binding should be created")
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.step(t)
		})
	}
}
