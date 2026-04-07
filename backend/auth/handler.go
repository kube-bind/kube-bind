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
	"crypto/rand"
	"crypto/sha256"
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

const maxCallbackFormSize int64 = 1 << 20 // 1 MiB

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
			ah.respondWithError(w, authReq.ClientType, "invalid JSON request", http.StatusBadRequest)
			return
		}
	} else {
		authReq = AuthorizeRequest{
			RedirectURL: params.RedirectURL,
			SessionID:   params.SessionID,
			ClusterID:   params.ClusterID,
			ClientType:  ClientType(params.ClientType),
			ConsumerID:  params.ConsumerID,
		}
	}

	logger.V(2).Info("AuthorizeRequest prepared", "authReq", authReq, "params", params)

	if authReq.RedirectURL == "" || authReq.SessionID == "" {
		logger.Error(errors.New("missing required parameters"), "failed to authorize")
		ah.respondWithError(w, authReq.ClientType, "missing redirect_url or session_id", http.StatusBadRequest)
		return
	}

	scopes := []string{"openid", "profile", "email", "offline_access", "groups"}
	dataCode, err := json.Marshal(authReq)
	if err != nil {
		logger.Error(err, "failed to marshal auth code")
		ah.respondWithError(w, authReq.ClientType, err.Error(), http.StatusInternalServerError)
		return
	}

	provider, err := ah.oidc.GetOIDCProvider(r.Context())
	if err != nil {
		logger.Error(err, "failed to get OIDC provider")
		ah.respondWithError(w, authReq.ClientType, err.Error(), http.StatusInternalServerError)
		return
	}

	encoded := base64.URLEncoding.EncodeToString(dataCode)

	verifier, challenge, err := generatePKCE()
	if err != nil {
		logger.Error(err, "failed to generate PKCE")
		ah.respondWithError(w, authReq.ClientType, "failed to generate PKCE", http.StatusInternalServerError)
		return
	}
	if err := ah.sessionStore.SavePKCEVerifier(r.Context(), authReq.SessionID, verifier); err != nil {
		logger.Error(err, "failed to store PKCE verifier")
		ah.respondWithError(w, authReq.ClientType, "failed to store PKCE verifier", http.StatusInternalServerError)
		return
	}

	opts := []oauth2.AuthCodeOption{
		oauth2.SetAuthURLParam("code_challenge", challenge),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
	}
	authURL := provider.OIDCProviderConfig(scopes).AuthCodeURL(encoded, opts...)

	http.Redirect(w, r, authURL, http.StatusFound)
}

func (ah *AuthHandler) HandleCallback(w http.ResponseWriter, r *http.Request) {
	logger := klog.FromContext(r.Context()).WithValues("method", r.Method, "url", r.URL.String())

	r.Body = http.MaxBytesReader(w, r.Body, maxCallbackFormSize)
	if err := r.ParseForm(); err != nil {
		logger.Error(err, "failed to parse form")
		ah.respondWithError(w, "", "failed to parse form", http.StatusBadRequest)
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
		ah.respondWithError(w, "", "invalid state parameter", http.StatusBadRequest)
		return
	}

	authCode := &AuthorizeRequest{}
	if err := json.Unmarshal(decoded, authCode); err != nil {
		logger.Error(err, "failed to unmarshal authCode")
		ah.respondWithError(w, authCode.ClientType, "invalid state format", http.StatusBadRequest)
		return
	}

	if errMsg := r.Form.Get("error"); errMsg != "" {
		desc := r.Form.Get("error_description")
		logger.Error(errors.New(errMsg), "OIDC provider returned error", "description", desc)
		ah.respondWithError(w, authCode.ClientType, fmt.Sprintf("%s: %s", errMsg, desc), http.StatusBadRequest)
		return
	}

	code := r.Form.Get("code")
	if code == "" {
		code = r.URL.Query().Get("code")
	}
	if code == "" {
		logger.Error(errors.New("missing code"), "no code in request")
		ah.respondWithError(w, authCode.ClientType, "no authorization code in request", http.StatusBadRequest)
		return
	}
	logger.V(2).Info("HandleCallback state unmarshaled", "authCode", authCode)

	provider, err := ah.oidc.GetOIDCProvider(r.Context())
	if err != nil {
		logger.Info("failed to get OIDC provider", "error", err)
		ah.respondWithError(w, authCode.ClientType, "failed to get OIDC provider", http.StatusInternalServerError)
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

	verifier, err := ah.sessionStore.LoadAndDeletePKCEVerifier(r.Context(), authCode.SessionID)
	if err != nil || verifier == "" {
		logger.Error(err, "PKCE verifier not found for session; cannot exchange code", "sessionID", authCode.SessionID)
		msg := "PKCE verifier not found. If you run multiple backend instances, use a shared session store (e.g. Redis) so the instance handling the callback can read the verifier stored at authorize time."
		ah.respondWithError(w, authCode.ClientType, msg, http.StatusBadRequest)
		return
	}

	exchangeOpts := []oauth2.AuthCodeOption{oauth2.VerifierOption(verifier)}
	token, err := provider.OIDCProviderConfig(nil).Exchange(ctx, code, exchangeOpts...)
	if err != nil {
		logger.Error(err, "failed to exchange token")
		ah.respondWithError(w, authCode.ClientType, "failed to exchange authorization code for token", http.StatusInternalServerError)
		return
	}

	sessionState, err := ah.createSessionState(authCode, token)
	if err != nil {
		logger.Error(err, "failed to create session sessionState")
		ah.respondWithError(w, authCode.ClientType, "failed to create user session", http.StatusInternalServerError)
		return
	}
	// Set session expiration and store in middleware
	err = ah.sessionStore.Save(r.Context(), sessionState)
	if err != nil {
		logger.Error(err, "failed to save session state")
		ah.respondWithError(w, authCode.ClientType, "failed to save session state", http.StatusInternalServerError)
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
			sessionState.Token.Groups,
			24*time.Hour, // 24 hours expiration
		)
		if err != nil {
			logger.Error(err, "failed to generate JWT token")
			ah.respondWithError(w, authCode.ClientType, "failed to generate JWT token", http.StatusInternalServerError)
			return
		}

		// Redirect to CLI redirect with JWT token
		parsedRedirectURL, err := url.Parse(authCode.RedirectURL)
		if err != nil {
			logger.Error(err, "failed to parse redirect URL")
			ah.respondWithError(w, authCode.ClientType, "failed to parse redirect URL", http.StatusInternalServerError)
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
		ah.respondWithError(w, authCode.ClientType, "failed to encode session cookie", http.StatusInternalServerError)
		return
	}

	secure := false
	http.SetCookie(w, session.MakeCookie(r, cookieName, encoded, secure, 1*time.Hour))

	clientParams := &client.ClientParameters{
		ClusterID:   authCode.ClusterID,
		RedirectURL: authCode.RedirectURL,
		SessionID:   authCode.SessionID,
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

func generatePKCE() (string, string, error) {
	data := make([]byte, 32)
	if _, err := rand.Read(data); err != nil {
		return "", "", err
	}
	verifier := base64.RawURLEncoding.EncodeToString(data)
	hash := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(hash[:])
	return verifier, challenge, nil
}
