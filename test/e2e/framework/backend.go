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
	"encoding/base64"
	"net"
	"testing"

	"github.com/gorilla/securecookie"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/require"
	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"

	backend "github.com/kube-bind/kube-bind/backend"
	"github.com/kube-bind/kube-bind/backend/options"
	"github.com/kube-bind/kube-bind/deploy/crd"
	kubebindv1alpha2 "github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2"
)

func InstallKubebindCRDs(t testing.TB, clientConfig *rest.Config) {
	crdClient, err := apiextensionsclient.NewForConfig(clientConfig)
	require.NoError(t, err)
	err = crd.Create(t.Context(),
		crdClient.ApiextensionsV1().CustomResourceDefinitions(),
		metav1.GroupResource{Group: kubebindv1alpha2.GroupName, Resource: "clusterbindings"},
		metav1.GroupResource{Group: kubebindv1alpha2.GroupName, Resource: "apiserviceexports"},
		metav1.GroupResource{Group: kubebindv1alpha2.GroupName, Resource: "apiservicenamespaces"},
		metav1.GroupResource{Group: kubebindv1alpha2.GroupName, Resource: "apiserviceexportrequests"},
		metav1.GroupResource{Group: kubebindv1alpha2.GroupName, Resource: "boundschemas"},
	)
	require.NoError(t, err)
}

func StartBackend(t testing.TB, args ...string) (net.Addr, *backend.Server) {
	signingKey := securecookie.GenerateRandomKey(32)
	require.NotEmpty(t, signingKey, "error creating signing key")
	encryptionKey := securecookie.GenerateRandomKey(32)
	require.NotEmpty(t, encryptionKey, "error creating encryption key")

	return StartBackendWithoutDefaultArgs(t, append([]string{
		"--oidc-issuer-url=http://127.0.0.1:5556/dex",
		"--cookie-signing-key=" + base64.StdEncoding.EncodeToString(signingKey),
		"--cookie-encryption-key=" + base64.StdEncoding.EncodeToString(encryptionKey),
	}, args...)...)
}

func StartBackendWithoutDefaultArgs(t testing.TB, args ...string) (net.Addr, *backend.Server) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	fs := pflag.NewFlagSet("backend", pflag.ContinueOnError)
	opts := options.NewOptions()
	opts.AddFlags(fs)
	err := fs.Parse(args)
	require.NoError(t, err)

	// use a random port via an explicit listener. Then add a kube-bind-<port> client to dex
	// with the callback URL set to the listener's address.
	opts.Serve.Listener, err = net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	addr := opts.Serve.Listener.Addr()

	dexId, dexSecret := CreateDexClient(t, addr)
	opts.OIDC.IssuerClientID = dexId
	opts.OIDC.IssuerClientSecret = dexSecret
	opts.OIDC.CallbackURL = "http://" + addr.String() + "/callback"

	// Skip name conflict validation - when run in-process with multiple
	// controllers they all will register the same metric names, which
	// causes subsequent controllers to fail.
	opts.ExtraOptions.TestingSkipNameValidation = true

	completed, err := opts.Complete()
	require.NoError(t, err)
	require.NoError(t, completed.Validate())

	config, err := backend.NewConfig(completed)
	require.NoError(t, err)

	server, err := backend.NewServer(ctx, config)
	require.NoError(t, err)

	err = server.Run(ctx)
	require.NoError(t, err)
	t.Logf("backend listening on %s", addr)

	return addr, server
}
