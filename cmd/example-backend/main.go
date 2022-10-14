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
	"fmt"
	"os"
	"time"

	"github.com/spf13/pflag"

	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apiextensionsinformers "k8s.io/apiextensions-apiserver/pkg/client/informers/externalversions"
	genericapiserver "k8s.io/apiserver/pkg/server"
	kubeinformers "k8s.io/client-go/informers"
	kubernetesclient "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"

	"github.com/kube-bind/kube-bind/cmd/example-backend/options"
	"github.com/kube-bind/kube-bind/contrib/example-backend/controllers/clusterbinding"
	"github.com/kube-bind/kube-bind/contrib/example-backend/controllers/serviceexport"
	"github.com/kube-bind/kube-bind/contrib/example-backend/controllers/serviceexportresource"
	"github.com/kube-bind/kube-bind/contrib/example-backend/controllers/servicenamespace"
	examplehttp "github.com/kube-bind/kube-bind/contrib/example-backend/http"
	examplekube "github.com/kube-bind/kube-bind/contrib/example-backend/kubernetes"
	bindclient "github.com/kube-bind/kube-bind/pkg/client/clientset/versioned"
	bindinformers "github.com/kube-bind/kube-bind/pkg/client/informers/externalversions"
)

func main() {
	ctx := genericapiserver.SetupSignalContext()
	defer klog.Flush()

	fs := pflag.NewFlagSet("example-backend", pflag.ContinueOnError)

	opts := options.NewOptions()
	opts.AddFlags(fs)
	if err := fs.Parse(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v", err) // nolint: errcheck
		os.Exit(1)
	}
	if err := opts.Complete(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v", err) // nolint: errcheck
		os.Exit(1)
	}
	if err := opts.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v", err) // nolint: errcheck
		os.Exit(1)
	}

	// setup rest client
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	rules.ExplicitPath = opts.KubeConfig
	cfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, nil).ClientConfig()
	if err != nil {
		klog.Fatalf("Error building kubeconfig: %s", err.Error())
		os.Exit(1)
	}
	cfg = rest.CopyConfig(cfg)
	cfg = rest.AddUserAgent(cfg, "kube-bind-example-backend")

	// construct informer factories
	bindClient, err := bindclient.NewForConfig(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error building bind client: %v", err) // nolint: errcheck
		os.Exit(1)
	}
	kubeClient, err := kubernetesclient.NewForConfig(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error building kubernetes client: %v", err) // nolint: errcheck
		os.Exit(1)
	}
	apiextensionClient, err := apiextensionsclient.NewForConfig(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error building apiextension client: %v", err) // nolint: errcheck
		os.Exit(1)
	}
	kubeInformers := kubeinformers.NewSharedInformerFactory(kubeClient, time.Minute*30)
	bindInformers := bindinformers.NewSharedInformerFactory(bindClient, time.Minute*30)
	apiextensionInformers := apiextensionsinformers.NewSharedInformerFactory(apiextensionClient, time.Minute*30)

	// setup oidc backend
	oidcProvider, err := examplehttp.NewOIDCServiceProvider(
		opts.OIDC.IssuerClientID,
		opts.OIDC.IssuerClientSecret,
		opts.OIDC.CallbackURL,
		opts.OIDC.IssuerURL,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error setting up OIDC: %v", err) // nolint: errcheck
		os.Exit(1)
	}
	mgr, err := examplekube.NewKubernetesManager(
		opts.NamespacePrefix,
		opts.PrettyName,
		cfg,
		kubeInformers.Core().V1().Namespaces(),
		bindInformers.KubeBind().V1alpha1().APIServiceExports(),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error setting up Kubernetes Manager: %v", err) // nolint: errcheck
		os.Exit(1)
	}
	handler, err := examplehttp.NewHandler(
		oidcProvider,
		opts.OIDC.CallbackURL,
		opts.PrettyName,
		mgr,
		apiextensionInformers.Apiextensions().V1().CustomResourceDefinitions().Lister(),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error setting up HTTP Handler: %v", err) // nolint: errcheck
		os.Exit(1)
	}
	server, err := examplehttp.NewServer(opts.ListenIP, opts.ListenPort, handler)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error setting up HTTP Server: %v", err) // nolint: errcheck
		os.Exit(1)
	}

	// construct controllers
	clusterBindingCtrl, err := clusterbinding.NewController(
		cfg,
		bindInformers.KubeBind().V1alpha1().ClusterBindings(),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error setting up ClusterBinding Controller: %v", err) // nolint: errcheck
		os.Exit(1)
	}
	servicenamespaceCtrl, err := servicenamespace.NewController(cfg,
		bindInformers.KubeBind().V1alpha1().APIServiceNamespaces(),
		bindInformers.KubeBind().V1alpha1().ClusterBindings(),
		bindInformers.KubeBind().V1alpha1().APIServiceExports(),
		kubeInformers.Core().V1().Namespaces(),
		kubeInformers.Rbac().V1().Roles(),
		kubeInformers.Rbac().V1().RoleBindings(),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error setting up APIServiceNamespace Controller: %v", err) // nolint: errcheck
		os.Exit(1)
	}
	serviceexportCtrl, err := serviceexport.NewController(cfg,
		bindInformers.KubeBind().V1alpha1().APIServiceExports(),
		bindInformers.KubeBind().V1alpha1().APIServiceExportResources(),
		apiextensionInformers.Apiextensions().V1().CustomResourceDefinitions(),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error setting up APIServiceExport Controller: %v", err) // nolint: errcheck
		os.Exit(1)
	}
	serviceexportresourceCtrl, err := serviceexportresource.NewController(cfg,
		bindInformers.KubeBind().V1alpha1().APIServiceExports(),
		bindInformers.KubeBind().V1alpha1().APIServiceExportResources(),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error setting up APIServiceExportResource Controller: %v", err) // nolint: errcheck
		os.Exit(1)
	}

	// start informer factories
	kubeInformers.Start(ctx.Done())
	bindInformers.Start(ctx.Done())
	apiextensionInformers.Start(ctx.Done())

	kubeInformers.WaitForCacheSync(ctx.Done())
	bindInformers.WaitForCacheSync(ctx.Done())
	apiextensionInformers.WaitForCacheSync(ctx.Done())

	// start controllers
	go clusterBindingCtrl.Start(ctx, 1)
	go servicenamespaceCtrl.Start(ctx, 1)
	go serviceexportCtrl.Start(ctx, 1)
	go serviceexportresourceCtrl.Start(ctx, 1)

	go func() {
		<-ctx.Done()
		os.Exit(1)
	}()
	server.Start()
}
