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
	"fmt"
	"net"
	"testing"
	"time"

	dexapi "github.com/dexidp/dex/api/v2"
	"github.com/gorilla/securecookie"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	grpcinsecure "google.golang.org/grpc/credentials/insecure"

	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"

	backend "github.com/kube-bind/kube-bind/contrib/example-backend"
	"github.com/kube-bind/kube-bind/contrib/example-backend/options"
	"github.com/kube-bind/kube-bind/deploy/crd"
	kubebindv1alpha1 "github.com/kube-bind/kube-bind/pkg/apis/kubebind/v1alpha1"
)

func StartBackend(t *testing.T, clientConfig *rest.Config, args ...string) (net.Addr, *backend.Server) {
	signingKey := securecookie.GenerateRandomKey(32)
	if len(signingKey) == 0 {
		panic("error creating signing key")
	}

	return StartBackendWithoutDefaultArgs(t, clientConfig, append([]string{
		"--oidc-issuer-client-secret=ZXhhbXBsZS1hcHAtc2VjcmV0",
		"--oidc-issuer-client-id=kube-bind",
		"--oidc-issuer-url=http://127.0.0.1:5556/dex",
		"--cookie-signing-key=" + base64.StdEncoding.EncodeToString(signingKey),
	}, args...)...)
}

func StartBackendWithoutDefaultArgs(t *testing.T, clientConfig *rest.Config, args ...string) (net.Addr, *backend.Server) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	crdClient, err := apiextensionsclient.NewForConfig(clientConfig)
	require.NoError(t, err)
	err = crd.Create(ctx,
		crdClient.ApiextensionsV1().CustomResourceDefinitions(),
		metav1.GroupResource{Group: kubebindv1alpha1.GroupName, Resource: "clusterbindings"},
		metav1.GroupResource{Group: kubebindv1alpha1.GroupName, Resource: "apiserviceexports"},
		metav1.GroupResource{Group: kubebindv1alpha1.GroupName, Resource: "apiservicenamespaces"},
		metav1.GroupResource{Group: kubebindv1alpha1.GroupName, Resource: "apiserviceexportrequests"},
		metav1.GroupResource{Group: kubebindv1alpha1.GroupName, Resource: "apiserviceexporttemplates"},
	)
	require.NoError(t, err)

	fs := pflag.NewFlagSet("example-backend", pflag.ContinueOnError)
	options := options.NewOptions()
	options.AddFlags(fs)
	err = fs.Parse(args)
	require.NoError(t, err)

	// use a random port via an explicit listener. Then add a kube-bind-<port> client to dex
	// with the callback URL set to the listener's address.
	options.Serve.Listener, err = net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	addr := options.Serve.Listener.Addr()
	_, port, err := net.SplitHostPort(addr.String())
	require.NoError(t, err)
	options.OIDC.IssuerClientID = "kube-bind-" + port
	createDexClient(t, addr)

	completed, err := options.Complete()
	require.NoError(t, err)

	config, err := backend.NewConfig(completed)
	require.NoError(t, err)

	server, err := backend.NewServer(config)
	require.NoError(t, err)

	server.OptionallyStartInformers(ctx)
	err = server.Run(ctx)
	require.NoError(t, err)
	t.Logf("backend listening on %s", addr)

	return addr, server
}

func createDexClient(t *testing.T, addr net.Addr) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	_, port, err := net.SplitHostPort(addr.String())
	require.NoError(t, err)
	conn, err := grpc.Dial("127.0.0.1:5557", grpc.WithTransportCredentials(grpcinsecure.NewCredentials()))
	require.NoError(t, err)
	defer conn.Close()
	client := dexapi.NewDexClient(conn)

	_, err = client.CreateClient(ctx, &dexapi.CreateClientReq{
		Client: &dexapi.Client{
			Id:           "kube-bind-" + port,
			Secret:       "ZXhhbXBsZS1hcHAtc2VjcmV0",
			RedirectUris: []string{fmt.Sprintf("http://%s/callback", addr)},
			Public:       true,
			Name:         "kube-bind on port " + port,
		},
	})
	require.NoError(t, err)

	t.Cleanup(func() {
		ctx, cancel := context.WithDeadline(context.Background(), metav1.Now().Add(10*time.Second))
		defer cancel()
		conn, err := grpc.Dial("127.0.0.1:5557", grpc.WithTransportCredentials(grpcinsecure.NewCredentials()))
		require.NoError(t, err)
		_, err = dexapi.NewDexClient(conn).DeleteClient(ctx, &dexapi.DeleteClientReq{Id: "kube-bind-" + port})
		require.NoError(t, err)
	})
}
