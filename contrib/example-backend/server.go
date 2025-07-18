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
	"log"
	"net"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog/v2"

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

	Controllers
}

type Controllers struct {
	ClusterBinding       *clusterbinding.Controller
	ServiceNamespace     *servicenamespace.Controller
	ServiceExport        *serviceexport.Controller
	ServiceExportRequest *serviceexportrequest.Controller
}

func NewServer(config *Config) (*Server, error) {
	s := &Server{
		Config: config,
	}

	var err error
	s.WebServer, err = examplehttp.NewServer(config.Options.Serve)
	if err != nil {
		return nil, fmt.Errorf("error setting up HTTP Server: %w", err)
	}

	// setup oidc backend
	callback := config.Options.OIDC.CallbackURL
	if callback == "" {
		callback = fmt.Sprintf("http://%s/callback", s.WebServer.Addr().String())
	}
	s.OIDC, err = examplehttp.NewOIDCServiceProvider(
		config.Options.OIDC.IssuerClientID,
		config.Options.OIDC.IssuerClientSecret,
		callback,
		config.Options.OIDC.IssuerURL,
	)
	if err != nil {
		return nil, fmt.Errorf("error setting up OIDC: %w", err)
	}
	s.Kubernetes, err = examplekube.NewKubernetesManager(
		config.Options.NamespacePrefix,
		config.Options.PrettyName,
		config.ClientConfig,
		config.Options.ExternalAddress,
		config.Options.ExternalCA,
		config.Options.TLSExternalServerName,
		config.KubeInformers.Core().V1().Namespaces(),
		config.BindInformers.KubeBind().V1alpha2().APIServiceExports(),
	)
	if err != nil {
		return nil, fmt.Errorf("error setting up Kubernetes Manager: %w", err)
	}

	signingKey, err := base64.StdEncoding.DecodeString(config.Options.Cookie.SigningKey)
	if err != nil {
		return nil, fmt.Errorf("error creating signing key: %w", err)
	}

	var encryptionKey []byte
	if config.Options.Cookie.EncryptionKey != "" {
		var err error
		encryptionKey, err = base64.StdEncoding.DecodeString(config.Options.Cookie.EncryptionKey)
		if err != nil {
			return nil, fmt.Errorf("error creating encryption key: %w", err)
		}
	}

	handler, err := examplehttp.NewHandler(
		s.OIDC,
		config.Options.OIDC.AuthorizeURL,
		callback,
		config.Options.PrettyName,
		config.Options.TestingAutoSelect,
		signingKey,
		encryptionKey,
		kubebindv1alpha2.InformerScope(config.Options.ConsumerScope),
		s.Kubernetes,
		config.ApiextensionsInformers.Apiextensions().V1().CustomResourceDefinitions().Lister(),
	)
	if err != nil {
		return nil, fmt.Errorf("error setting up HTTP Handler: %w", err)
	}
	handler.AddRoutes(s.WebServer.Router)

	// construct controllers
	s.ClusterBinding, err = clusterbinding.NewController(
		config.ClientConfig,
		kubebindv1alpha2.InformerScope(config.Options.ConsumerScope),
		config.BindInformers.KubeBind().V1alpha2().ClusterBindings(),
		config.BindInformers.KubeBind().V1alpha2().APIServiceExports(),
		config.KubeInformers.Rbac().V1().ClusterRoles(),
		config.KubeInformers.Rbac().V1().ClusterRoleBindings(),
		config.KubeInformers.Rbac().V1().RoleBindings(),
		config.KubeInformers.Core().V1().Namespaces(),
	)
	if err != nil {
		return nil, fmt.Errorf("error setting up ClusterBinding Controller: %v", err)
	}
	s.ServiceNamespace, err = servicenamespace.NewController(
		config.ClientConfig,
		kubebindv1alpha2.InformerScope(config.Options.ConsumerScope),
		config.BindInformers.KubeBind().V1alpha2().APIServiceNamespaces(),
		config.BindInformers.KubeBind().V1alpha2().ClusterBindings(),
		config.BindInformers.KubeBind().V1alpha2().APIServiceExports(),
		config.KubeInformers.Core().V1().Namespaces(),
		config.KubeInformers.Rbac().V1().Roles(),
		config.KubeInformers.Rbac().V1().RoleBindings(),
	)
	if err != nil {
		return nil, fmt.Errorf("error setting up APIServiceNamespace Controller: %w", err)
	}
	s.ServiceExport, err = serviceexport.NewController(
		config.ClientConfig,
		config.BindInformers.KubeBind().V1alpha2().APIServiceExports(),
		config.ApiextensionsInformers.Apiextensions().V1().CustomResourceDefinitions(),
	)
	if err != nil {
		return nil, fmt.Errorf("error setting up APIServiceExport Controller: %w", err)
	}
	s.ServiceExportRequest, err = serviceexportrequest.NewController(
		config.ClientConfig,
		kubebindv1alpha2.InformerScope(config.Options.ConsumerScope),
		kubebindv1alpha2.Isolation(config.Options.ClusterScopedIsolation),
		config.BindInformers.KubeBind().V1alpha2().APIServiceExportRequests(),
		config.BindInformers.KubeBind().V1alpha2().APIServiceExports(),
		config.ApiextensionsInformers.Apiextensions().V1().CustomResourceDefinitions(),
		config.BindInformers.KubeBind().V1alpha2().APIResourceSchemas(),
	)
	if err != nil {
		return nil, fmt.Errorf("error setting up ServiceExportRequest Controller: %w", err)
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
	dynamicClient, err := dynamic.NewForConfig(s.Config.ClientConfig)
	if err != nil {
		return err
	}

	if err := deploy.Bootstrap(ctx, s.Config.KubeClient.Discovery(), dynamicClient, sets.Set[string]{}); err != nil {
		return err
	}

	// start controllers
	go s.ServiceExport.Start(ctx, 1)
	go s.ServiceNamespace.Start(ctx, 1)
	go s.ClusterBinding.Start(ctx, 1)
	go s.ServiceExportRequest.Start(ctx, 1)

	go func() {
		<-ctx.Done()
		log.Println("Context done")
	}()
	return s.WebServer.Start(ctx)
}
