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
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/securecookie"
	"golang.org/x/oauth2"
	"k8s.io/klog/v2"

	"github.com/kube-bind/kube-bind/backend/client"
	"github.com/kube-bind/kube-bind/backend/session"
)

type OIDCProvider interface {
	GetOIDCProvider(ctx context.Context) (*OIDCServiceProvider, error)
}

type AuthHandlerInterface interface {
	HandleAuthorize(w http.ResponseWriter, r *http.Request)
	HandleCallback(w http.ResponseWriter, r *http.Request)
}

type AuthHandler struct {
	oidc                OIDCProvider
	jwtService          *JWTService
	cookieSigningKey    []byte
	cookieEncryptionKey []byte
	sessionStore        session.Store
	tokenExpiry         time.Duration
}

func NewAuthHandler(oidc OIDCProvider, jwtService *JWTService, cookieSigningKey, cookieEncryptionKey []byte, sessionStore session.Store, tokenExpiry time.Duration) *AuthHandler {
	return &AuthHandler{
		oidc:                oidc,
		jwtService:          jwtService,
		cookieSigningKey:    cookieSigningKey,
		cookieEncryptionKey: cookieEncryptionKey,
		sessionStore:        sessionStore,
		tokenExpiry:         tokenExpiry,
	}
}

func (ah *AuthHandler) HandleAuthorize(w http.ResponseWriter, r *http.Request) {
	logger := klog.FromContext(r.Context()).WithValues("method", r.Method, "url", r.URL.String())

	params := client.GetQueryParams(r)

	var authReq AuthorizeRequest
	if r.Method == http.MethodPost {
		if err := json.NewDecoder(r.Body).Decode(&authReq); err != nil {
			http.Error(w, "invalid JSON request", http.StatusBadRequest)
			return
		}
	} else {
		authReq = AuthorizeRequest{
			RedirectURL:           params.RedirectURL,
			ClientSideRedirectURL: params.ClientSideRedirectURL,
			SessionID:             params.SessionID,
			ClusterID:             params.ClusterID,
			ClientType:            ClientType(params.ClientType),
		}
	}

	if authReq.RedirectURL == "" || authReq.SessionID == "" {
		logger.Error(errors.New("missing required parameters"), "failed to authorize")
		ah.respondWithError(w, authReq.ClientType, "missing redirect_url or session_id", http.StatusBadRequest)
		return
	}

	scopes := []string{"openid", "profile", "email", "offline_access", "groups"}
	dataCode, err := json.Marshal(authReq)
	if err != nil {
		logger.Info("failed to marshal auth code", "error", err)
		ah.respondWithError(w, authReq.ClientType, err.Error(), http.StatusInternalServerError)
		return
	}

	provider, err := ah.oidc.GetOIDCProvider(r.Context())
	if err != nil {
		logger.Info("failed to get OIDC provider", "error", err)
		ah.respondWithError(w, authReq.ClientType, err.Error(), http.StatusInternalServerError)
		return
	}

	encoded := base64.URLEncoding.EncodeToString(dataCode)
	authURL := provider.OIDCProviderConfig(scopes).AuthCodeURL(encoded)

	http.Redirect(w, r, authURL, http.StatusFound)
}

func (ah *AuthHandler) HandleCallback(w http.ResponseWriter, r *http.Request) {
	logger := klog.FromContext(r.Context()).WithValues("method", r.Method, "url", r.URL.String())
	if errMsg := r.Form.Get("error"); errMsg != "" {
		logger.Error(errors.New(errMsg), "failed to authorize")
		http.Error(w, errMsg+": "+r.Form.Get("error_description"), http.StatusBadRequest)
		return
	}

	code := r.Form.Get("code")
	if code == "" {
		code = r.URL.Query().Get("code")
	}
	if code == "" {
		logger.Error(errors.New("missing code"), "no code in request")
		http.Error(w, fmt.Sprintf("no code in request: %q", r.Form), http.StatusBadRequest)
		return
	}

	state := r.Form.Get("state")
	if state == "" {
		state = r.URL.Query().Get("state")
	}

	// URL decode the state parameter first (in case it's URL encoded)
	if decodedState, err := url.QueryUnescape(state); err == nil {
		state = decodedState
	}

	decoded, err := base64.URLEncoding.DecodeString(state)
	if err != nil {
		logger.Error(err, "failed to decode state")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	authCode := &AuthorizeRequest{}
	if err := json.Unmarshal(decoded, authCode); err != nil {
		logger.Error(err, "failed to unmarshal authCode")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	provider, err := ah.oidc.GetOIDCProvider(r.Context())
	if err != nil {
		logger.Info("failed to get OIDC provider", "error", err)
		ah.respondWithError(w, authCode.ClientType, err.Error(), http.StatusInternalServerError)
		return
	}

	// Create context with custom HTTP client if TLS config is available
	ctx := r.Context()
	if tlsConfig := provider.GetTLSConfig(); tlsConfig != nil {
		client := &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: tlsConfig,
			},
		}
		ctx = context.WithValue(ctx, oauth2.HTTPClient, client)
	}

	token, err := provider.OIDCProviderConfig(nil).Exchange(ctx, code)
	if err != nil {
		logger.Error(err, "failed to exchange token")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	sessionState, err := ah.createSessionState(authCode, token)
	if err != nil {
		logger.Error(err, "failed to create session sessionState")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	// Set session expiration and store in middleware
	err = ah.sessionStore.Save(sessionState)
	if err != nil {
		logger.Error(err, "failed to save session state")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Detect client type from redirect URL or user agent
	clientType := authCode.ClientType

	if clientType == ClientTypeCLI {
		// Generate JWT token for CLI
		jwtToken, err := ah.jwtService.GenerateToken(
			sessionState.Token.Subject,
			sessionState.Token.Issuer,
			sessionState.SessionID,
			sessionState.ClusterID,
			sessionState.RedirectURL,
			24*time.Hour, // 24 hours expiration
		)
		if err != nil {
			logger.Error(err, "failed to generate JWT token")
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		// Redirect to CLI redirect with JWT token
		parsedRedirectURL, err := url.Parse(authCode.RedirectURL)
		if err != nil {
			logger.Error(err, "failed to parse redirect URL")
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		values := parsedRedirectURL.Query()
		values.Add("access_token", jwtToken)
		values.Add("token_type", "Bearer")
		values.Add("expires_in", "86400") // 24 hours
		if authCode.ClusterID != "" {
			values.Add("cluster_id", authCode.ClusterID)
		}
		parsedRedirectURL.RawQuery = values.Encode()

		http.Redirect(w, r, parsedRedirectURL.String(), http.StatusFound)
		return
	}

	// UI flow - set cookie and redirect to UI
	cookieName := ah.generateCookieName(authCode.ClusterID)

	s := securecookie.New(ah.cookieSigningKey, ah.cookieEncryptionKey)
	encoded, err := s.Encode(cookieName, sessionState)
	if err != nil {
		logger.Error(err, "failed to encode secure session cookie")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	secure := false
	http.SetCookie(w, session.MakeCookie(r, cookieName, encoded, secure, 1*time.Hour))

	clientParams := &client.ClientParameters{
		ClusterID:             authCode.ClusterID,
		ClientSideRedirectURL: authCode.ClientSideRedirectURL,
		RedirectURL:           authCode.RedirectURL,
		SessionID:             authCode.SessionID,
	}
	url := clientParams.WithParams(authCode.RedirectURL)

	http.Redirect(w, r, url, http.StatusFound)
}

func (ah *AuthHandler) respondWithError(w http.ResponseWriter, clientType ClientType, message string, statusCode int) {
	if clientType != ClientTypeCLI {
		http.Error(w, message, statusCode)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	err := json.NewEncoder(w).Encode(AuthResponse{
		Success: false,
		Error:   message,
	})
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

func (ah *AuthHandler) generateCookieName(clusterID string) string {
	if clusterID == "" {
		return "kube-bind"
	}
	return fmt.Sprintf("kube-bind-%s", clusterID)
}

func (ah *AuthHandler) createSessionState(authCode *AuthorizeRequest, token *oauth2.Token) (*session.State, error) {
	jwtStr, ok := token.Extra("id_token").(string)
	if !ok {
		return nil, errors.New("no id_token value found in token")
	}

	jwt, err := ah.unwrapJWT(jwtStr)
	if err != nil {
		return nil, fmt.Errorf("failed to unpack ID token: %w", err)
	}

	var idToken struct {
		Subject string   `json:"sub"`
		Issuer  string   `json:"iss"`
		Groups  []string `json:"groups,omitempty"`
	}
	if err := json.Unmarshal(jwt, &idToken); err != nil {
		return nil, fmt.Errorf("failed to parse ID token: %w", err)
	}

	s := &session.State{
		Token: session.TokenInfo{
			Subject: idToken.Subject,
			Issuer:  idToken.Issuer,
			Groups:  idToken.Groups,
		},
		SessionID:   authCode.SessionID,
		ClusterID:   authCode.ClusterID,
		RedirectURL: authCode.RedirectURL,
	}
	s.SetExpiration(ah.tokenExpiry)
	return s, nil
}

func (ah *AuthHandler) unwrapJWT(p string) ([]byte, error) {
	parts := strings.Split(p, ".")
	if len(parts) < 2 {
		return nil, fmt.Errorf("OIDC: malformed JWT, expected 3 parts, got %d", len(parts))
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("OIDC: malformed JWT payload: %w", err)
	}
	return payload, nil
}
