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

	kcpapisv1alpha2 "github.com/kcp-dev/kcp/sdk/apis/apis/v1alpha2"
	kcpclientset "github.com/kcp-dev/kcp/sdk/client/clientset/versioned/cluster"
	kcptestinghelpers "github.com/kcp-dev/kcp/sdk/testing/helpers"
	kcptestingserver "github.com/kcp-dev/kcp/sdk/testing/server"
	"github.com/kcp-dev/logicalcluster/v3"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/rest"

	"github.com/kube-bind/kube-bind/contrib/kcp/bootstrap"
	"github.com/kube-bind/kube-bind/contrib/kcp/bootstrap/options"
	"github.com/kube-bind/kube-bind/test/e2e/framework"
)

func wsConfig(t testing.TB, server kcptestingserver.RunningServer, workspace logicalcluster.Path) (*rest.Config, string) {
	cfg := server.BaseConfig(t)
	cfg.Host += "/clusters/" + workspace.String()

	kubeconfig := framework.WriteKubeconfig(t,
		framework.RestToKubeconfig(cfg, "default"),
		workspace.Base()+".kubeconfig",
	)

	return cfg, kubeconfig
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
	t.Log("Bootstrapping kcp")

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
