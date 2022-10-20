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

package framework

import (
	"context"
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/require"

	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"

	"github.com/kube-bind/kube-bind/deploy/crd"
	kubebindv1alpha1 "github.com/kube-bind/kube-bind/pkg/apis/kubebind/v1alpha1"
	"github.com/kube-bind/kube-bind/pkg/konnector"
	"github.com/kube-bind/kube-bind/pkg/konnector/options"
)

func StartKonnector(t *testing.T, clientConfig *rest.Config, args ...string) *konnector.Server {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	crdClient, err := apiextensionsclient.NewForConfig(clientConfig)
	require.NoError(t, err)
	err = crd.Create(ctx,
		crdClient.ApiextensionsV1().CustomResourceDefinitions(),
		metav1.GroupResource{Group: kubebindv1alpha1.GroupName, Resource: "apiservicebindings"},
	)
	require.NoError(t, err)

	fs := pflag.NewFlagSet("konnector", pflag.ContinueOnError)
	options := options.NewOptions()
	options.AddFlags(fs)
	err = fs.Parse(args)
	require.NoError(t, err)

	completed, err := options.Complete()
	require.NoError(t, err)

	config, err := konnector.NewConfig(completed)
	require.NoError(t, err)

	server, err := konnector.NewServer(config)
	require.NoError(t, err)
	prepared, err := server.PrepareRun(ctx)
	require.NoError(t, err)

	prepared.OptionallyStartInformers(ctx)
	go func() {
		err := prepared.Run(ctx)
		select {
		case <-ctx.Done():
		default:
			require.NoError(t, err)
		}
	}()

	return server
}
