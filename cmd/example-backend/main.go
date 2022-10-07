/*
Copyright 2022 The Kubectl Bind contributors.

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

package main

import (
	"flag"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"

	http2 "github.com/kube-bind/kube-bind/contrib/example-backend/http"
	"github.com/kube-bind/kube-bind/contrib/example-backend/kubernetes"
	"github.com/kube-bind/kube-bind/pkg/apis/kubebind/v1alpha1"
)

type backendOpts struct {
	listenIP   string
	listenPort int

	// at the moment there is no TLS configs but should be added later on.
	oidcIssuerClientID     string
	oidcIssuerClientSecret string
	oidcIssuerURL          string

	kubeconfig  string
	clusterName string
	namespace   string
}

var (
	scheme = runtime.NewScheme()
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(v1alpha1.AddToScheme(scheme))
}

func newBackendOptions() *backendOpts {
	opts := &backendOpts{}
	flag.StringVar(&opts.listenIP, "listenIP", "127.0.0.1", "The host IP where the backend is running")
	flag.IntVar(&opts.listenPort, "listenPort", 8080, "The host port where the backend is running")
	flag.StringVar(&opts.oidcIssuerClientID, "oidc-issuer-client-id", "", "Issuer client ID")
	flag.StringVar(&opts.oidcIssuerClientSecret, "oidc-issuer-client-secret", "", "OpenID client secret")
	flag.StringVar(&opts.oidcIssuerURL, "oidc-issuer-url", "", "Callback URL for OpenID responses.")
	flag.StringVar(&opts.kubeconfig, "kubeconfig", "", "path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&opts.namespace, "namespace", "kube-system", "the namespace where the biding resources are created at.")
	flag.StringVar(&opts.clusterName, "cluster-name", "", "the name of the cluster where kube-bind apis will run.")

	flag.Parse()

	return opts
}

func main() {
	opts := newBackendOptions()
	oidcRedirect := fmt.Sprintf("http://%s:%v/callback", opts.listenIP, opts.listenPort)

	oidcProvider, err := http2.NewOIDCServiceProvider(opts.oidcIssuerClientID,
		opts.oidcIssuerClientSecret,
		oidcRedirect,
		opts.oidcIssuerURL)

	if err != nil {
		klog.Fatalf("error building the oidc provider: %v", err)
	}

	cfg, err := clientcmd.BuildConfigFromFlags("", opts.kubeconfig)
	if err != nil {
		klog.Fatalf("error building kubeconfig: %v", err)
	}
	cfg.ContentConfig.GroupVersion = &schema.GroupVersion{Group: v1alpha1.GroupName, Version: v1alpha1.GroupVersion}
	cfg.APIPath = "/apis"
	cfg.NegotiatedSerializer = serializer.NewCodecFactory(scheme)
	cfg.UserAgent = rest.DefaultKubernetesUserAgent()

	mgr, err := kubernetes.NewKubernetesManager(cfg, opts.clusterName, opts.namespace)
	if err != nil {
		klog.Fatalf("error building kubernetes manager: %v", err)
	}

	handler, err := http2.NewHandler(oidcProvider, mgr)
	if err != nil {
		klog.Fatalf("error building the http handler: %v", err)
	}

	server, err := http2.NewServer(opts.listenIP, opts.listenPort, handler)
	if err != nil {
		klog.Fatalf("failed to start the server: %v", err)
	}

	server.Start()
}
