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

package auth

import (
	"context"
	"crypto/tls"
	"net/http"
	"time"

	oidc "github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

type AuthResponse struct {
	Success bool                   `json:"success"`
	Data    interface{}            `json:"data,omitempty"`
	Error   string                 `json:"error,omitempty"`
	Message string                 `json:"message,omitempty"`
	Meta    map[string]interface{} `json:"meta,omitempty"`
}

type TokenResponse struct {
	AccessToken  string    `json:"access_token"`
	TokenType    string    `json:"token_type"`
	ExpiresIn    int64     `json:"expires_in"`
	ExpiresAt    time.Time `json:"expires_at"`
	Scope        string    `json:"scope,omitempty"`
	RefreshToken string    `json:"refresh_token,omitempty"`

	ClusterID string `json:"cluster_id,omitempty"`
}

// TODO: We should remove client_side_redirect_url.
// https://github.com/kube-bind/kube-bind/issues/362

type AuthorizeRequest struct {
	RedirectURL           string     `json:"redirect_url" form:"redirect_url"`
	ClientSideRedirectURL string     `json:"client_side_redirect_url" form:"client_side_redirect_url"`
	SessionID             string     `json:"session_id" form:"session_id"`
	ClusterID             string     `json:"cluster_id" form:"cluster_id"`
	ClientType            ClientType `json:"client_type" form:"client_type"`
	ConsumerID            string     `json:"consumer_id" form:"consumer_id"`
}

type CallbackRequest struct {
	Code             string `json:"code" form:"code"`
	State            string `json:"state" form:"state"`
	Error            string `json:"error,omitempty" form:"error"`
	ErrorDescription string `json:"error_description,omitempty" form:"error_description"`
}

type OIDCServiceProvider struct {
	clientID     string
	clientSecret string
	redirectURI  string
	issuerURL    string

	verifier  *oidc.IDTokenVerifier
	provider  *oidc.Provider
	tlsConfig *tls.Config
}

func NewOIDCServiceProvider(ctx context.Context, clientID, clientSecret, redirectURI, issuerURL string) (*OIDCServiceProvider, error) {
	provider, err := oidc.NewProvider(ctx, issuerURL)
	if err != nil {
		return nil, err
	}

	return &OIDCServiceProvider{
		clientID:     clientID,
		clientSecret: clientSecret,
		redirectURI:  redirectURI,
		issuerURL:    issuerURL,
		provider:     provider,
		verifier:     provider.Verifier(&oidc.Config{ClientID: clientID}),
		tlsConfig:    nil,
	}, nil
}

func NewOIDCServiceProviderWithTLS(
	ctx context.Context,
	clientID, clientSecret, redirectURI, issuerURL string,
	tlsConfig *tls.Config,
) (*OIDCServiceProvider, error) {
	// Create a custom HTTP client that trusts the TLS config
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
	}

	// Create context with the custom client
	ctxWithClient := oidc.ClientContext(ctx, client)

	provider, err := oidc.NewProvider(ctxWithClient, issuerURL)
	if err != nil {
		return nil, err
	}

	return &OIDCServiceProvider{
		clientID:     clientID,
		clientSecret: clientSecret,
		redirectURI:  redirectURI,
		issuerURL:    issuerURL,
		provider:     provider,
		verifier:     provider.Verifier(&oidc.Config{ClientID: clientID}),
		tlsConfig:    tlsConfig,
	}, nil
}

func (o *OIDCServiceProvider) OIDCProviderConfig(scopes []string) *oauth2.Config {
	config := &oauth2.Config{
		ClientID:     o.clientID,
		ClientSecret: o.clientSecret,
		Endpoint:     o.provider.Endpoint(),
		RedirectURL:  o.redirectURI,
		Scopes:       scopes,
	}

	return config
}

func (o *OIDCServiceProvider) GetTLSConfig() *tls.Config {
	return o.tlsConfig
}
