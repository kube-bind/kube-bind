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

package backend

import (
	"context"
	"encoding/base64"
	"fmt"
	"net"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/dynamic"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"github.com/kube-bind/kube-bind/contrib/example-backend/controllers/clusterbinding"
	"github.com/kube-bind/kube-bind/contrib/example-backend/controllers/serviceexport"
	"github.com/kube-bind/kube-bind/contrib/example-backend/controllers/serviceexportrequest"
	"github.com/kube-bind/kube-bind/contrib/example-backend/controllers/servicenamespace"
	"github.com/kube-bind/kube-bind/contrib/example-backend/deploy"
	examplehttp "github.com/kube-bind/kube-bind/contrib/example-backend/http"
	examplekube "github.com/kube-bind/kube-bind/contrib/example-backend/kubernetes"
	kubebindv1alpha2 "github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2"
)

type Server struct {
	Config *Config

	OIDC       *examplehttp.OIDCServiceProvider
	Kubernetes *examplekube.Manager
	WebServer  *examplehttp.Server
	Manager    manager.Manager

	Controllers
}

type Controllers struct {
	ClusterBinding       *clusterbinding.ClusterBindingReconciler
	ServiceExport        *serviceexport.APIServiceExportReconciler
	ServiceExportRequest *serviceexportrequest.APIServiceExportRequestReconciler

	ServiceNamespace *servicenamespace.APIServiceNamespaceReconciler
}

func NewServer(ctx context.Context, c *Config) (*Server, error) {
	s := &Server{
		Config: c,
	}

	var err error
	s.WebServer, err = examplehttp.NewServer(c.Options.Serve)
	if err != nil {
		return nil, fmt.Errorf("error setting up HTTP Server: %w", err)
	}

	// setup oidc backend
	callback := c.Options.OIDC.CallbackURL
	if callback == "" {
		callback = fmt.Sprintf("http://%s/callback", s.WebServer.Addr().String())
	}
	s.OIDC, err = examplehttp.NewOIDCServiceProvider(
		c.Options.OIDC.IssuerClientID,
		c.Options.OIDC.IssuerClientSecret,
		callback,
		c.Options.OIDC.IssuerURL,
	)
	if err != nil {
		return nil, fmt.Errorf("error setting up OIDC: %w", err)
	}
	s.Kubernetes, err = examplekube.NewKubernetesManager(
		c.Options.NamespacePrefix,
		c.Options.PrettyName,
		c.ClientConfig,
		c.Options.ExternalAddress,
		c.Options.ExternalCA,
		c.Options.TLSExternalServerName,
		c.KubeInformers.Core().V1().Namespaces(),
		c.BindInformers.KubeBind().V1alpha2().APIServiceExports(),
	)
	if err != nil {
		return nil, fmt.Errorf("error setting up Kubernetes Manager: %w", err)
	}

	signingKey, err := base64.StdEncoding.DecodeString(c.Options.Cookie.SigningKey)
	if err != nil {
		return nil, fmt.Errorf("error creating signing key: %w", err)
	}

	var encryptionKey []byte
	if c.Options.Cookie.EncryptionKey != "" {
		var err error
		encryptionKey, err = base64.StdEncoding.DecodeString(c.Options.Cookie.EncryptionKey)
		if err != nil {
			return nil, fmt.Errorf("error creating encryption key: %w", err)
		}
	}

	handler, err := examplehttp.NewHandler(
		s.OIDC,
		c.Options.OIDC.AuthorizeURL,
		callback,
		c.Options.PrettyName,
		c.Options.TestingAutoSelect,
		signingKey,
		encryptionKey,
		kubebindv1alpha2.InformerScope(c.Options.ConsumerScope),
		s.Kubernetes,
		c.ApiextensionsInformers.Apiextensions().V1().CustomResourceDefinitions().Lister(),
	)
	if err != nil {
		return nil, fmt.Errorf("error setting up HTTP Handler: %w", err)
	}
	handler.AddRoutes(s.WebServer.Router)

	// Set up controller-runtime manager
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		return nil, fmt.Errorf("error adding client-go scheme: %w", err)
	}
	if err := apiextensionsv1.AddToScheme(scheme); err != nil {
		return nil, fmt.Errorf("error adding apiextensions scheme: %w", err)
	}
	if err := kubebindv1alpha2.AddToScheme(scheme); err != nil {
		return nil, fmt.Errorf("error adding kubebind scheme: %w", err)
	}

	s.Manager, err = ctrl.NewManager(c.ClientConfig, ctrl.Options{
		Controller: config.Controller{
			SkipNameValidation: ptr.To(true), // TODO(mjudeikis): Currently tests are setting came controller many times. Refactor this in follow up.
		},
		Metrics: metricsserver.Options{
			BindAddress: "0",
		},
		Scheme: scheme,
	})
	if err != nil {
		return nil, fmt.Errorf("error setting up controller manager: %w", err)
	}

	// construct controllers
	s.ClusterBinding, err = clusterbinding.NewClusterBindingReconciler(
		s.Manager.GetClient(),
		s.Manager.GetScheme(),
		c.ClientConfig,
		kubebindv1alpha2.InformerScope(c.Options.ConsumerScope),
		s.Manager.GetCache(),
	)
	if err != nil {
		return nil, fmt.Errorf("error setting up ClusterBinding Controller: %w", err)
	}

	// Register the ClusterBinding controller with the manager
	if err := s.ClusterBinding.SetupWithManager(s.Manager); err != nil {
		return nil, fmt.Errorf("error setting up ClusterBinding controller with manager: %w", err)
	}

	// construct APIServiceExport controller with controller-runtime
	s.ServiceExport, err = serviceexport.NewAPIServiceExportReconciler(
		ctx,
		s.Manager.GetClient(),
		s.Manager.GetScheme(),
		c.ClientConfig,
		s.Manager.GetCache(),
	)
	if err != nil {
		return nil, fmt.Errorf("error setting up APIServiceExport Controller: %w", err)
	}

	// Register the APIServiceExport controller with the manager
	if err := s.ServiceExport.SetupWithManager(s.Manager); err != nil {
		return nil, fmt.Errorf("error setting up APIServiceExport controller with manager: %w", err)
	}

	s.ServiceNamespace, err = servicenamespace.NewAPIServiceNamespaceReconciler(
		ctx,
		s.Manager.GetClient(),
		s.Manager.GetScheme(),
		c.ClientConfig,
		s.Manager.GetCache(),
		kubebindv1alpha2.InformerScope(c.Options.ConsumerScope),
		kubebindv1alpha2.Isolation(c.Options.ClusterScopedIsolation),
	)
	if err != nil {
		return nil, fmt.Errorf("error setting up APIServiceNamespace Controller: %w", err)
	}

	// Register the APIServiceNamespace controller with the manager
	if err := s.ServiceNamespace.SetupWithManager(s.Manager); err != nil {
		return nil, fmt.Errorf("error setting up APIServiceNamespace controller with manager: %w", err)
	}
	s.ServiceExportRequest, err = serviceexportrequest.NewAPIServiceExportRequestReconciler(
		ctx,
		s.Manager.GetClient(),
		s.Manager.GetScheme(),
		c.ClientConfig,
		s.Manager.GetCache(),
		kubebindv1alpha2.InformerScope(c.Options.ConsumerScope),
		kubebindv1alpha2.Isolation(c.Options.ClusterScopedIsolation),
	)
	if err != nil {
		return nil, fmt.Errorf("error setting up ServiceExportRequest Controller: %w", err)
	}

	// Register the ServiceExportRequest controller with the manager
	if err := s.ServiceExportRequest.SetupWithManager(s.Manager); err != nil {
		return nil, fmt.Errorf("error setting up ServiceExportRequest controller with manager: %w", err)
	}

	return s, nil
}

func (s *Server) OptionallyStartInformers(ctx context.Context) {
	logger := klog.FromContext(ctx)

	// start informer factories
	logger.Info("starting informers")
	s.Config.KubeInformers.Start(ctx.Done())
	s.Config.BindInformers.Start(ctx.Done())
	s.Config.ApiextensionsInformers.Start(ctx.Done())
	kubeSynced := s.Config.KubeInformers.WaitForCacheSync(ctx.Done())
	kubeBindSynced := s.Config.BindInformers.WaitForCacheSync(ctx.Done())
	apiextensionsSynced := s.Config.ApiextensionsInformers.WaitForCacheSync(ctx.Done())

	logger.Info("local informers are synced",
		"kubeSynced", fmt.Sprintf("%v", kubeSynced),
		"kubeBindSynced", fmt.Sprintf("%v", kubeBindSynced),
		"apiextensionsSynced", fmt.Sprintf("%v", apiextensionsSynced),
	)
}

func (s *Server) Addr() net.Addr {
	return s.WebServer.Addr()
}

func (s *Server) Run(ctx context.Context) error {
	logger := klog.FromContext(ctx)
	dynamicClient, err := dynamic.NewForConfig(s.Config.ClientConfig)
	if err != nil {
		return err
	}

	if err := deploy.Bootstrap(ctx, s.Config.KubeClient.Discovery(), dynamicClient, sets.Set[string]{}); err != nil {
		return err
	}

	// start controller-runtime manager after bootstrap completes
	go func() {
		if err := s.Manager.Start(ctx); err != nil {
			logger.Error(err, "Failed to start controller manager")
		}
	}()

	go func() {
		<-ctx.Done()
		logger.Info("Context done")
	}()
	return s.WebServer.Start(ctx)
}
