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

package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apiextensionsinformers "k8s.io/apiextensions-apiserver/pkg/client/informers/externalversions"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	genericapiserver "k8s.io/apiserver/pkg/server"
	kubeinformers "k8s.io/client-go/informers"
	kubernetesclient "k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"

	"github.com/kube-bind/kube-bind/contrib/example-backend/controllers/serviceexport"
	"github.com/kube-bind/kube-bind/contrib/example-backend/controllers/serviceexportresource"
	"github.com/kube-bind/kube-bind/contrib/example-backend/controllers/servicenamespace"
	examplehttp "github.com/kube-bind/kube-bind/contrib/example-backend/http"
	examplekube "github.com/kube-bind/kube-bind/contrib/example-backend/kubernetes"
	kubebindv1alpha1 "github.com/kube-bind/kube-bind/pkg/apis/kubebind/v1alpha1"
	bindclient "github.com/kube-bind/kube-bind/pkg/client/clientset/versioned"
	bindinformers "github.com/kube-bind/kube-bind/pkg/client/informers/externalversions"
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

	utilruntime.Must(kubebindv1alpha1.AddToScheme(scheme))
}

func newBackendOptions() *backendOpts {
	opts := &backendOpts{}
	flag.StringVar(&opts.listenIP, "listen-ip", "127.0.0.1", "The host IP where the backend is running")
	flag.IntVar(&opts.listenPort, "listen-port", 8080, "The host port where the backend is running")
	flag.StringVar(&opts.oidcIssuerClientID, "oidc-issuer-client-id", "", "Issuer client ID")
	flag.StringVar(&opts.oidcIssuerClientSecret, "oidc-issuer-client-secret", "", "OpenID client secret")
	flag.StringVar(&opts.oidcIssuerURL, "oidc-issuer-url", "", "Callback URL for OpenID responses.")
	flag.StringVar(&opts.kubeconfig, "kubeconfig", "", "path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&opts.namespace, "namespace", "kube-system", "the namespace where the biding resources are created at.")
	flag.StringVar(&opts.clusterName, "cluster-name", "", "the name of the cluster where kube-bind apis will run.")

	klog.InitFlags(nil)
	flag.Set("v", "2") // nolint:errcheck

	flag.Parse()

	return opts
}

func main() {
	ctx := genericapiserver.SetupSignalContext()
	defer klog.Flush()

	opts := newBackendOptions()
	oidcRedirect := fmt.Sprintf("http://%s:%v/callback", opts.listenIP, opts.listenPort)

	oidcProvider, err := examplehttp.NewOIDCServiceProvider(opts.oidcIssuerClientID,
		opts.oidcIssuerClientSecret,
		oidcRedirect,
		opts.oidcIssuerURL)

	if err != nil {
		klog.Fatalf("error building the oidc provider: %v", err)
	}

	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	rules.ExplicitPath = opts.kubeconfig
	cfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, nil).ClientConfig()
	if err != nil {
		klog.Fatalf("error building kubeconfig: %v", err)
	}
	cfg.UserAgent = rest.DefaultKubernetesUserAgent()

	mgr, err := examplekube.NewKubernetesManager(cfg, opts.clusterName, opts.namespace)
	if err != nil {
		klog.Fatalf("error building kubernetes manager: %v", err)
	}

	handler, err := examplehttp.NewHandler(oidcProvider, mgr)
	if err != nil {
		klog.Fatalf("error building the http handler: %v", err)
	}

	server, err := examplehttp.NewServer(opts.listenIP, opts.listenPort, handler)
	if err != nil {
		klog.Fatalf("failed to start the server: %v", err)
	}

	// construct informer factories
	bindClient, err := bindclient.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("error building bind client: %v", err)
	}
	kubeClient, err := kubernetesclient.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("error building kubernetes client: %v", err)
	}
	apiextensionClient, err := apiextensionsclient.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("error building apiextension client: %v", err)
	}
	kubeInformers := kubeinformers.NewSharedInformerFactory(kubeClient, time.Minute*30)
	bindInformers := bindinformers.NewSharedInformerFactory(bindClient, time.Minute*30)
	apiextensionInformers := apiextensionsinformers.NewSharedInformerFactory(apiextensionClient, time.Minute*30)

	// construct controllers
	servicenamespaceCtrl, err := servicenamespace.NewController(cfg,
		bindInformers.KubeBind().V1alpha1().APIServiceNamespaces(),
		bindInformers.KubeBind().V1alpha1().ClusterBindings(),
		bindInformers.KubeBind().V1alpha1().APIServiceExports(),
		kubeInformers.Core().V1().Namespaces(),
		kubeInformers.Rbac().V1().Roles(),
		kubeInformers.Rbac().V1().RoleBindings(),
	)
	if err != nil {
		klog.Fatalf("error building the APIServiceNamespace controller: %v", err)
	}
	serviceexportCtrl, err := serviceexport.NewController(cfg,
		bindInformers.KubeBind().V1alpha1().APIServiceExports(),
		bindInformers.KubeBind().V1alpha1().APIServiceExportResources(),
		apiextensionInformers.Apiextensions().V1().CustomResourceDefinitions(),
	)
	if err != nil {
		klog.Fatalf("error building the APIServiceExport controller: %v", err)
	}
	serviceexportresourceCtrl, err := serviceexportresource.NewController(cfg,
		bindInformers.KubeBind().V1alpha1().APIServiceExports(),
		bindInformers.KubeBind().V1alpha1().APIServiceExportResources(),
	)
	if err != nil {
		klog.Fatalf("error building the APIServiceExport controller: %v", err)
	}

	// start informer factories
	kubeInformers.Start(ctx.Done())
	bindInformers.Start(ctx.Done())
	apiextensionInformers.Start(ctx.Done())

	kubeInformers.WaitForCacheSync(ctx.Done())
	bindInformers.WaitForCacheSync(ctx.Done())
	apiextensionInformers.WaitForCacheSync(ctx.Done())

	// start controllers
	go servicenamespaceCtrl.Start(ctx, 1)
	go serviceexportCtrl.Start(ctx, 1)
	go serviceexportresourceCtrl.Start(ctx, 1)

	go func() {
		<-ctx.Done()
		os.Exit(1)
	}()
	server.Start()
}
