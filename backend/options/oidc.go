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

package options

import (
	"crypto/tls"
	"fmt"
	"net"

	"github.com/spf13/pflag"

	"github.com/kube-bind/kube-bind/backend/oidc"
	kubebindv1alpha2 "github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2"
)

type OIDC struct {
	Type               string
	IssuerClientID     string
	IssuerClientSecret string
	IssuerURL          string
	CallbackURL        string
	AuthorizeURL       string
	CAFile             string

	// TLSConfig is set if an embedded OIDC server is used.
	TLSConfig  *tls.Config
	OIDCServer *oidc.Server
}

func NewOIDC() *OIDC {
	return &OIDC{
		Type: string(kubebindv1alpha2.OIDCProviderTypeExternal),
	}
}

func (options *OIDC) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&options.IssuerClientID, "oidc-issuer-client-id", options.IssuerClientID, "Issuer client ID")
	fs.StringVar(&options.IssuerClientSecret, "oidc-issuer-client-secret", options.IssuerClientSecret, "OpenID client secret")
	fs.StringVar(&options.IssuerURL, "oidc-issuer-url", options.IssuerURL, "Callback URL for OpenID responses.")
	fs.StringVar(&options.CallbackURL, "oidc-callback-url", options.CallbackURL, "OpenID callback URL")
	fs.StringVar(&options.AuthorizeURL, "oidc-authorize-url", options.AuthorizeURL, "OpenID authorize URL")
	fs.StringVar(&options.CAFile, "oidc-ca-file", options.CAFile, "Path to a CA bundle to use when verifying the OIDC provider's TLS certificate.")
	fs.StringVar(&options.Type, "oidc-type", options.Type, "Type of OIDC provider (embedded or external)")
}

func (options *OIDC) Complete(listener net.Listener) error {
	if options.Type == string(kubebindv1alpha2.OIDCProviderTypeEmbedded) {
		oidcServer, err := oidc.New(options.CAFile, listener, options.IssuerURL)
		if err != nil {
			return err
		}
		options.OIDCServer = oidcServer

		cfg, err := oidcServer.Config(options.CallbackURL, options.IssuerURL)
		if err != nil {
			return err
		}
		options.TLSConfig = oidcServer.TLSConfig()
		options.IssuerURL = cfg.Issuer
		options.IssuerClientID = cfg.ClientID
		options.IssuerClientSecret = cfg.ClientSecret
		// This should be provided from outside, but in embedded - we detect.
		options.CallbackURL = cfg.CallbackURL
	}

	if options.CAFile != "" {
		tlsConfig, err := oidc.LoadTLSConfig(options.CAFile)
		if err != nil {
			return fmt.Errorf("failed to load OIDC CA file: %w", err)
		}
		options.TLSConfig = tlsConfig
	}

	return nil
}

func (options *OIDC) Validate() error {
	if options.IssuerClientID == "" {
		return fmt.Errorf("OIDC issuer client ID cannot be empty")
	}
	if options.IssuerClientSecret == "" {
		return fmt.Errorf("OIDC issuer client secret cannot be empty")
	}
	if options.IssuerURL == "" {
		return fmt.Errorf("OIDC issuer URL cannot be empty")
	}
	if options.CallbackURL == "" {
		return fmt.Errorf("OIDC callback URL cannot be empty")
	}
	if options.CAFile != "" && options.TLSConfig != nil {
		return fmt.Errorf("cannot use both CA file and embedded OIDC server")
	}

	return nil
}
