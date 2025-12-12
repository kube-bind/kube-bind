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
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/securecookie"
	"k8s.io/klog/v2"

	"github.com/kube-bind/kube-bind/backend/kubernetes"
	"github.com/kube-bind/kube-bind/backend/session"
)

type contextKey string

const (
	AuthContextKey contextKey = "auth_context"
)

type ClientType string

const (
	ClientTypeUI  ClientType = "ui"
	ClientTypeCLI ClientType = "cli"
)

type AuthContext struct {
	SessionState *session.State
	ClientType   ClientType
	IsValid      bool
}

type AuthMiddleware struct {
	jwtService          *JWTService
	kubernetesMananger  *kubernetes.Manager
	cookieSigningKey    []byte
	cookieEncryptionKey []byte
	sessionStore        session.Store
}

func NewAuthMiddleware(
	jwtService *JWTService,
	cookieSigningKey, cookieEncryptionKey []byte,
	kubernetesMananger *kubernetes.Manager,
	sessionStore session.Store,
) *AuthMiddleware {
	return &AuthMiddleware{
		jwtService:          jwtService,
		cookieSigningKey:    cookieSigningKey,
		cookieEncryptionKey: cookieEncryptionKey,
		kubernetesMananger:  kubernetesMananger,
		sessionStore:        sessionStore,
	}
}

func (am *AuthMiddleware) AuthenticateRequest(next http.Handler) http.Handler {
	return am.authenticate(
		am.verifyState(
			am.authorizeK8S(next)),
	)
}

func (am *AuthMiddleware) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger := klog.FromContext(r.Context())

		authCtx := &AuthContext{
			IsValid: false,
		}

		authHeader := r.Header.Get("Authorization")

		if authHeader != "" {
			if strings.HasPrefix(authHeader, "Bearer ") {
				token := strings.TrimPrefix(authHeader, "Bearer ")
				if claims, err := am.jwtService.ValidateToken(token); err == nil {
					authCtx.SessionState = &session.State{
						Token: session.TokenInfo{
							Subject: claims.Subject,
							Issuer:  claims.Issuer,
						},
						SessionID:   claims.SessionID,
						ClusterID:   claims.ClusterID,
						RedirectURL: claims.RedirectURL,
					}
					authCtx.ClientType = ClientTypeCLI
				} else {
					logger.V(2).Info("Invalid JWT token", "error", err)
				}
			}
		}

		// Fall back to cookie authentication (for UI clients)
		if authCtx.SessionState == nil {
			cookieName := "kube-bind"
			if r.URL.Query().Get("cluster_id") != "" {
				cookieName = "kube-bind-" + r.URL.Query().Get("cluster_id")
			}
			if cookie, err := r.Cookie(cookieName); err == nil {
				s := securecookie.New(am.cookieSigningKey, am.cookieEncryptionKey)
				state := &session.State{}
				if err := s.Decode(cookieName, cookie.Value, state); err == nil {
					// Only set as valid after all checks pass
					authCtx.SessionState = state
					authCtx.ClientType = ClientTypeUI
				}
			} else {
				logger.V(2).Info("Failed to decode session cookie", "error", err)
			}
		}

		// Add auth context to request context
		ctx := context.WithValue(r.Context(), AuthContextKey, authCtx)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (am *AuthMiddleware) verifyState(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger := klog.FromContext(r.Context())

		authCtx := GetAuthContext(r.Context())
		if authCtx.SessionState == nil {
			logger.V(2).Info("No session state found in auth context")
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		state := authCtx.SessionState
		// Validate session fields are present
		if state.Token.Subject == "" || state.Token.Issuer == "" || state.SessionID == "" {
			logger.V(2).Info("Invalid session state: missing required fields")
		} else if state.IsExpired() {
			logger.V(2).Info("Session expired", "sessionID", state.SessionID)
		} else if !am.isValidSession(state.SessionID) {
			logger.V(2).Info("Session ID not found or expired", "sessionID", state.SessionID)
		} else {
			authCtx.IsValid = true
		}
		ctx := context.WithValue(r.Context(), AuthContextKey, authCtx)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// isValidSession checks if a session ID exists and hasn't expired
func (am *AuthMiddleware) isValidSession(sessionID string) bool {
	sessionInfo, err := am.sessionStore.Load(sessionID)
	if err != nil {
		return false
	}
	return time.Now().Before(sessionInfo.ExpiresAt)
}

func (am *AuthMiddleware) authorizeK8S(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger := klog.FromContext(r.Context())

		authCtx := GetAuthContext(r.Context())
		if !authCtx.IsValid { // should not happen if AuthenticateRequest is used before
			logger.V(2).Info("Authentication context is not valid")
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		// Authorize against Kubernetes RBAC
		err := am.kubernetesMananger.AuthorizeRequest(r.Context(), authCtx.SessionState.Token.Subject, authCtx.SessionState.ClusterID, r.Method, r.URL.Path)
		if err != nil {
			logger.V(2).Info("Kubernetes RBAC authorization failed", "error", err)
			http.Error(w, "cluster does not have required permissions to bind", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func GetAuthContext(ctx context.Context) *AuthContext {
	if authCtx, ok := ctx.Value(AuthContextKey).(*AuthContext); ok {
		return authCtx
	}
	return &AuthContext{ClientType: ClientTypeUI, IsValid: false}
}

func RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authCtx := GetAuthContext(r.Context())
		if !authCtx.IsValid {
			if authCtx.ClientType == ClientTypeCLI {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				err := json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized", "message": "Valid JWT token required"})
				if err != nil {
					http.Error(w, "internal error", http.StatusInternalServerError)
				}
			} else {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
			}
			return
		}
		next.ServeHTTP(w, r)
	})
}
