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

package http

import (
	"context"

	oidc "github.com/coreos/go-oidc"
	"golang.org/x/oauth2"
)

type oidcServiceProvider struct {
	clientID     string
	clientSecret string
	redirectURI  string
	issuerURL    string

	verifier *oidc.IDTokenVerifier
	provider *oidc.Provider
}

func NewOIDCServiceProvider(clientID, clientSecret, redirectURI, issuerURL string) (*oidcServiceProvider, error) {
	provider, err := oidc.NewProvider(context.TODO(), issuerURL)
	if err != nil {
		return nil, err
	}

	return &oidcServiceProvider{
		clientID:     clientID,
		clientSecret: clientSecret,
		redirectURI:  redirectURI,
		issuerURL:    issuerURL,
		provider:     provider,
		verifier:     provider.Verifier(&oidc.Config{ClientID: clientID}),
	}, nil
}

func (o *oidcServiceProvider) OIDCProviderConfig(scopes []string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     o.clientID,
		ClientSecret: o.clientSecret,
		Endpoint:     o.provider.Endpoint(),
		RedirectURL:  o.redirectURI,
		Scopes:       scopes,
	}
}
