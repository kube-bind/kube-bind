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
	"os"

	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	mcreconcile "sigs.k8s.io/multicluster-runtime/pkg/reconcile"

	"github.com/kube-bind/kube-bind/backend/controllers/clusterbinding"
	"github.com/kube-bind/kube-bind/backend/controllers/serviceexport"
	"github.com/kube-bind/kube-bind/backend/controllers/serviceexportrequest"
	"github.com/kube-bind/kube-bind/backend/controllers/servicenamespace"
	http "github.com/kube-bind/kube-bind/backend/http"
	kube "github.com/kube-bind/kube-bind/backend/kubernetes"
	kubebindv1alpha2 "github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2"
)

type Server struct {
	Config *Config

	OIDC       *http.OIDCServiceProvider
	Kubernetes *kube.Manager
	WebServer  *http.Server

	Controllers
}

type Controllers struct {
	ClusterBinding       *clusterbinding.ClusterBindingReconciler
	ServiceExport        *serviceexport.APIServiceExportReconciler
	ServiceExportRequest *serviceexportrequest.APIServiceExportRequestReconciler
	ServiceNamespace     *servicenamespace.APIServiceNamespaceReconciler
}

func NewServer(ctx context.Context, c *Config) (*Server, error) {
	s := &Server{
		Config: c,
	}

	var err error
	s.WebServer, err = http.NewServer(c.Options.Serve)
	if err != nil {
		return nil, fmt.Errorf("error setting up HTTP Server: %w", err)
	}

	// setup oidc backend
	callback := c.Options.OIDC.CallbackURL
	if callback == "" {
		callback = fmt.Sprintf("http://%s/callback", s.WebServer.Addr().String())
	}
	s.OIDC, err = http.NewOIDCServiceProvider(
		ctx,
		c.Options.OIDC.IssuerClientID,
		c.Options.OIDC.IssuerClientSecret,
		callback,
		c.Options.OIDC.IssuerURL,
	)
	if err != nil {
		return nil, fmt.Errorf("error setting up OIDC: %w", err)
	}
	s.Kubernetes, err = kube.NewKubernetesManager(
		ctx,
		c.Options.NamespacePrefix,
		c.Options.PrettyName,
		c.ExternalAddressGenerator,
		kubebindv1alpha2.InformerScope(c.Options.ExtraOptions.ConsumerScope),
		c.Options.ExternalCA,
		c.Options.TLSExternalServerName,
		s.Config.Manager,
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

	handler, err := http.NewHandler(
		s.OIDC,
		c.Options.OIDC.AuthorizeURL,
		callback,
		c.Options.PrettyName,
		c.Options.TestingAutoSelect,
		signingKey,
		encryptionKey,
		c.Options.SchemaSource,
		kubebindv1alpha2.InformerScope(c.Options.ConsumerScope),
		s.Kubernetes,
	)
	if err != nil {
		return nil, fmt.Errorf("error setting up HTTP Handler: %w", err)
	}
	handler.AddRoutes(s.WebServer.Router)

	opts := controller.TypedOptions[mcreconcile.Request]{
		SkipNameValidation: ptr.To(c.Options.TestingSkipNameValidation),
	}

	s.ClusterBinding, err = clusterbinding.NewClusterBindingReconciler(
		ctx,
		s.Config.Manager,
		opts,
		kubebindv1alpha2.InformerScope(c.Options.ConsumerScope),
	)
	if err != nil {
		return nil, fmt.Errorf("error setting up ClusterBinding Controller: %w", err)
	}

	// Register the ClusterBinding controller with the manager
	if err := s.ClusterBinding.SetupWithManager(s.Config.Manager); err != nil {
		return nil, fmt.Errorf("error setting up ClusterBinding controller with manager: %w", err)
	}

	// construct APIServiceExport controller with multicluster-runtime
	s.ServiceExport, err = serviceexport.NewAPIServiceExportReconciler(
		ctx,
		s.Config.Manager,
		opts,
	)
	if err != nil {
		return nil, fmt.Errorf("error setting up APIServiceExport Controller: %w", err)
	}

	// Register the APIServiceExport controller with the manager
	if err := s.ServiceExport.SetupWithManager(s.Config.Manager); err != nil {
		return nil, fmt.Errorf("error setting up APIServiceExport controller with manager: %w", err)
	}

	s.ServiceNamespace, err = servicenamespace.NewAPIServiceNamespaceReconciler(
		ctx,
		s.Config.Manager,
		opts,
		kubebindv1alpha2.InformerScope(c.Options.ConsumerScope),
		kubebindv1alpha2.Isolation(c.Options.ClusterScopedIsolation),
	)
	if err != nil {
		return nil, fmt.Errorf("error setting up APIServiceNamespace Controller: %w", err)
	}

	// Register the APIServiceNamespace controller with the manager
	if err := s.ServiceNamespace.SetupWithManager(s.Config.Manager); err != nil {
		return nil, fmt.Errorf("error setting up APIServiceNamespace controller with manager: %w", err)
	}
	s.ServiceExportRequest, err = serviceexportrequest.NewAPIServiceExportRequestReconciler(
		ctx,
		s.Config.Manager,
		opts,
		kubebindv1alpha2.InformerScope(c.Options.ConsumerScope),
		kubebindv1alpha2.Isolation(c.Options.ClusterScopedIsolation),
		c.Options.SchemaSource,
	)
	if err != nil {
		return nil, fmt.Errorf("error setting up ServiceExportRequest Controller: %w", err)
	}

	// Register the ServiceExportRequest controller with the manager
	if err := s.ServiceExportRequest.SetupWithManager(s.Config.Manager); err != nil {
		return nil, fmt.Errorf("error setting up ServiceExportRequest controller with manager: %w", err)
	}

	return s, nil
}

func (s *Server) Addr() net.Addr {
	return s.WebServer.Addr()
}

func (s *Server) Run(ctx context.Context) error {
	logger := klog.FromContext(ctx)

	if s.Config.Provider != nil {
		logger.Info("Starting provider")
		go func() {
			if err := s.Config.Provider.Start(ctx, s.Config.Manager); err != nil {
				logger.Error(err, "unable to run provider")
				os.Exit(1)
			}
		}()
	}

	// start controller-runtime manager after bootstrap completes
	go func() {
		if err := s.Config.Manager.Start(ctx); err != nil {
			logger.Error(err, "Failed to start controller manager")
		}
	}()

	go func() {
		<-ctx.Done()
		logger.Info("Context done")
	}()
	return s.WebServer.Start(ctx)
}
