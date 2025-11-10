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

package oidc

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/gorilla/mux"
	"github.com/xrstf/mockoidc"
)

type Server struct {
	server    *mockoidc.MockOIDC
	tlsConfig *tls.Config
}

func New(caBundleFile string, listener net.Listener, addrOverride string) (*Server, error) {
	// Add offline_access to supported scopes for refresh token support
	ensureOfflineAccessScope()
	var tlsConfig *tls.Config
	s := &Server{}
	if caBundleFile != "" {
		var err error
		tlsConfig, err = LoadTLSConfig(caBundleFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load CA bundle file: %w", err)
		}
		s.tlsConfig = tlsConfig
	}

	server, err := mockoidc.NewServer(&mockoidc.ServerConfig{
		TLSConfig:    tlsConfig,
		Listener:     listener,
		AddrOverride: addrOverride,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create mock OIDC server: %w", err)
	}
	s.server = server
	return s, nil
}

// Config gives the configuration for clients to connect to the embedded OIDC server.
type Config struct {
	ClientID     string
	ClientSecret string
	Issuer       string

	AccessTTL  time.Duration
	RefreshTTL time.Duration

	CodeChallengeMethodsSupported []string

	// CallbackURL and IssuerURL are kube-bind specific and must match API server endpoints.
	CallbackURL string
	IssuerURL   string
}

var ErrServerNotRunning = fmt.Errorf("embedded OIDC server is not running")

func (s *Server) TLSConfig() *tls.Config {
	return s.tlsConfig
}

func (s *Server) AddRoutes(mux *mux.Router) {
	s.server.AddRoutes(mux)
}

// URL returns the base URL of the embedded OIDC server.
func (s *Server) Config(callbackURL, issuerURL string) (*Config, error) {
	c := &Config{
		ClientID:     s.server.Config().ClientID,
		ClientSecret: s.server.Config().ClientSecret,
		Issuer:       issuerURL, // This overrided default fake OIDC issuer URL. Must match what it is served at.

		AccessTTL:  s.server.Config().AccessTTL,
		RefreshTTL: s.server.Config().RefreshTTL,

		CodeChallengeMethodsSupported: s.server.Config().CodeChallengeMethodsSupported,
		CallbackURL:                   callbackURL,
		IssuerURL:                     issuerURL,
	}
	spew.Dump(c)
	return c, nil
}

func LoadTLSConfig(caFile string) (*tls.Config, error) {
	caCertPool := x509.NewCertPool()
	caCert, err := os.ReadFile(caFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read CA file: %w", err)
	}
	if ok := caCertPool.AppendCertsFromPEM(caCert); !ok {
		return nil, fmt.Errorf("failed to append CA certs from PEM")
	}
	return &tls.Config{
		RootCAs:    caCertPool,
		MinVersion: tls.VersionTLS13,
	}, nil
}

// ensureOfflineAccessScope adds "offline_access" to mockoidc's supported scopes if not already present
func ensureOfflineAccessScope() {
	offlineAccess := "offline_access"

	// Check if offline_access is already in the supported scopes
	for _, scope := range mockoidc.ScopesSupported {
		if scope == offlineAccess {
			return // Already present
		}
	}

	// Add offline_access to the supported scopes
	mockoidc.ScopesSupported = append(mockoidc.ScopesSupported, offlineAccess)
}
