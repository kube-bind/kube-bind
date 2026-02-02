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
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/rest"

	bindapiservice "github.com/kube-bind/kube-bind/cli/pkg/kubectl/bind-apiservice/plugin"
	kubebindv1alpha2 "github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2"
	"github.com/kube-bind/kube-bind/test/e2e/framework"
)

func performBinding(
	t *testing.T,
	consumerCfg *rest.Config,
	templateRef string,
	crdName string,
	kubeBindConfig string,
) {
	// Implementation of the binding process using the provided parameters
	// This is a placeholder for the actual binding logic
	t.Logf("Performing binding with templateRef: %s", templateRef)

	// 1. Get APIServiceExportRequest from provider
	c := framework.GetKubeBindRestClient(t, kubeBindConfig)

	identity, err := uuid.NewUUID()
	require.NoError(t, err)

	bindResponse, err := c.Bind(t.Context(), &kubebindv1alpha2.BindableResourcesRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-binding",
		},
		Spec: kubebindv1alpha2.BindableResourcesRequestSpec{
			TemplateRef: kubebindv1alpha2.APIServiceExportTemplateRef{
				Name: templateRef,
			},
			ClusterIdentity: kubebindv1alpha2.ClusterIdentity{
				Identity: identity.String(),
			},
		},
	})
	require.NoError(t, err)
	require.NotNil(t, bindResponse)

	iostreams, _, _, _ := genericclioptions.NewTestIOStreams()
	binderOpts := &bindapiservice.BinderOptions{
		IOStreams:     iostreams,
		SkipKonnector: true,
	}

	binder := bindapiservice.NewBinder(consumerCfg, binderOpts)
	result, err := binder.BindFromResponse(t.Context(), bindResponse)
	require.NoError(t, err)
	require.Len(t, result, 1)

	t.Logf("Waiting for %s CRD to be created on consumer side", templateRef)
	crdClient := framework.ApiextensionsClient(t, consumerCfg).ApiextensionsV1().CustomResourceDefinitions()
	require.Eventually(t, func() bool {
		crds, err := crdClient.List(t.Context(), metav1.ListOptions{})
		if err != nil {
			return false
		}
		for _, crd := range crds.Items {
			if strings.Contains(crd.Name, crdName) {
				return true
			}
		}
		return false
	}, wait.ForeverTestTimeout, time.Millisecond*100, "waiting for %s CRD to be created on consumer side", crdName)
}
