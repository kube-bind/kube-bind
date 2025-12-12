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

package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	componentbaseversion "k8s.io/component-base/version"
	"k8s.io/klog/v2"

	"github.com/kube-bind/kube-bind/backend/auth"
	"github.com/kube-bind/kube-bind/backend/client"
	"github.com/kube-bind/kube-bind/backend/kubernetes"
	"github.com/kube-bind/kube-bind/backend/oidc"
	"github.com/kube-bind/kube-bind/backend/session"
	"github.com/kube-bind/kube-bind/backend/spaserver"
	bindversion "github.com/kube-bind/kube-bind/pkg/version"
	kubebindv1alpha2 "github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2"
	"github.com/kube-bind/kube-bind/web"
)

// See https://developers.google.com/web/fundamentals/performance/optimizing-content-efficiency/http-caching?hl=en
var noCacheHeaders = map[string]string{
	"Expires":         time.Unix(0, 0).Format(time.RFC1123),
	"Cache-Control":   "no-cache, no-store, must-revalidate, max-age=0",
	"X-Accel-Expires": "0", // https://www.nginx.com/resources/wiki/start/topics/examples/x-accel/
}

type handler struct {
	oidcProvider   auth.OIDCProvider
	authHandler    auth.AuthHandlerInterface
	authMiddleware *auth.AuthMiddleware

	scope              kubebindv1alpha2.InformerScope
	oidcAuthorizeURL   string
	backendCallbackURL string
	providerPrettyName string
	testingAutoSelect  string
	schemaSource       string

	cookieEncryptionKey []byte
	cookieSigningKey    []byte

	client      *http.Client
	kubeManager *kubernetes.Manager
	oidcServer  *oidc.Server

	frontend string
}

func NewHandler(
	oidcProvider auth.OIDCProvider,
	oidcServer *oidc.Server,
	oidcAuthorizeURL, backendCallbackURL, providerPrettyName, testingAutoSelect string,
	cookieSigningKey, cookieEncryptionKey []byte,
	schemaSource string,
	scope kubebindv1alpha2.InformerScope,
	mgr *kubernetes.Manager,
	frontend string,
) (*handler, error) {
	// Create JWT service for CLI authentication
	jwtService, err := auth.NewJWTService("kube-bind-backend")
	if err != nil {
		return nil, fmt.Errorf("failed to create JWT service: %w", err)
	}

	sessionStore := session.NewInMemoryStore()

	// Create auth middleware for request authentication
	authMiddleware := auth.NewAuthMiddleware(jwtService, cookieSigningKey, cookieEncryptionKey, mgr, sessionStore)

	// Create auth handler with OIDC provider
	authHandler := auth.NewAuthHandler(oidcProvider, jwtService, cookieSigningKey, cookieEncryptionKey, sessionStore)

	return &handler{
		oidcProvider:        oidcProvider,
		authHandler:         authHandler,
		authMiddleware:      authMiddleware,
		oidcAuthorizeURL:    oidcAuthorizeURL,
		backendCallbackURL:  backendCallbackURL,
		providerPrettyName:  providerPrettyName,
		testingAutoSelect:   testingAutoSelect,
		schemaSource:        schemaSource,
		frontend:            frontend,
		scope:               scope,
		client:              http.DefaultClient,
		kubeManager:         mgr,
		cookieSigningKey:    cookieSigningKey,
		cookieEncryptionKey: cookieEncryptionKey,
		oidcServer:          oidcServer,
	}, nil
}

func (h *handler) AddRoutes(mux *mux.Router) error {
	// Public API routes (no authentication required)
	mux.HandleFunc("/api/healthz", h.handleHealthz).Methods(http.MethodGet)
	mux.HandleFunc("/api/bindable-resources", h.handleBindableResources).Methods(http.MethodGet)

	// Generic authentication routes (support both UI and CLI)
	mux.HandleFunc("/api/authorize", h.authHandler.HandleAuthorize).Methods(http.MethodGet, http.MethodPost)
	mux.HandleFunc("/api/callback", h.authHandler.HandleCallback).Methods(http.MethodGet)
	mux.HandleFunc("/api/logout", h.handleLogout).Methods(http.MethodPost)

	mux.HandleFunc("/api/exports", h.handleServiceExport).Methods(http.MethodGet)
	mux.HandleFunc("/exports", h.handleExportsRedirect).Methods(http.MethodGet) // This provides HTTP 302 redirects for backwards compatibility with older CLI versions and in general nicer UX.

	// Protected API routes (require authentication)
	apiRouter := mux.PathPrefix("/api").Subrouter()
	apiRouter.Use(h.authMiddleware.AuthenticateRequest)

	apiRouter.Handle("/templates", auth.RequireAuth(http.HandlerFunc(h.handleTemplates))).Methods(http.MethodGet)
	apiRouter.Handle("/collections", auth.RequireAuth(http.HandlerFunc(h.handleCollections))).Methods(http.MethodGet)
	apiRouter.Handle("/bind", auth.RequireAuth(http.HandlerFunc(h.handleBind))).Methods(http.MethodPost)
	apiRouter.Handle("/ping", auth.RequireAuth(http.HandlerFunc(h.handlePing))).Methods(http.MethodGet)

	if h.oidcServer != nil {
		h.oidcServer.AddRoutes(mux)
	}

	switch {
	// Development mode: proxy to frontend dev server
	case strings.HasPrefix(h.frontend, "http://"):
		spaserver, err := spaserver.NewSPAReverseProxyServer(h.frontend)
		if err != nil {
			return err
		}
		mux.PathPrefix("/").Handler(spaserver)
	default:
		fs := web.GetFileSystem()
		mux.PathPrefix("/").Handler(spaserver.NewSPAFileServer(fs))
	}
	return nil
}

func (h *handler) handleHealthz(w http.ResponseWriter, r *http.Request) {
	prepareNoCache(w)
	w.WriteHeader(http.StatusOK)
}

func (h *handler) handlePing(w http.ResponseWriter, r *http.Request) {
	prepareNoCache(w)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("pong")) //nolint:errcheck
}

func (h *handler) handleLogout(w http.ResponseWriter, r *http.Request) {
	prepareNoCache(w)

	// Get cluster ID from query parameter if present
	clusterID := r.URL.Query().Get("cluster_id")

	// Determine cookie name based on cluster ID
	cookieName := "kube-bind"
	if clusterID != "" {
		cookieName = "kube-bind-" + clusterID
	}

	// Create an expired cookie to clear the existing one
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     "/",
		Domain:   "",
		Expires:  time.Unix(0, 0), // Set to Unix epoch (Jan 1, 1970) to expire immediately
		HttpOnly: true,
		Secure:   r.TLS != nil, // Set secure flag if HTTPS
		SameSite: http.SameSiteLaxMode,
	})

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("logged out")) //nolint:errcheck
}

func (h *handler) handleExportsRedirect(w http.ResponseWriter, r *http.Request) {
	logger := getLogger(r)
	redirectURL := "/api/exports"

	// Preserve query parameters
	if r.URL.RawQuery != "" {
		redirectURL += "?" + r.URL.RawQuery
	}

	logger.Info("redirecting CLI exports request", "from", r.URL.Path, "to", redirectURL)
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

func (h *handler) handleServiceExport(w http.ResponseWriter, r *http.Request) {
	logger := klog.FromContext(r.Context()).WithValues("method", r.Method, "url", r.URL.String())
	params := client.GetQueryParams(r)

	oidcAuthorizeURL := params.WithParams(fmt.Sprintf("http://%s/api/authorize", r.Host))

	ver, err := bindversion.BinaryVersion(componentbaseversion.Get().GitVersion)
	if err != nil {
		logger.Error(err, "failed to parse version %q", componentbaseversion.Get().GitVersion)
		ver = "v0.0.0"
	}
	prettyName := h.providerPrettyName
	if prettyName == "" {
		prettyName = "backend"
	}
	provider := &kubebindv1alpha2.BindingProvider{
		TypeMeta: metav1.TypeMeta{
			APIVersion: kubebindv1alpha2.GroupVersion,
			Kind:       "BindingProvider",
		},
		Version:            ver,
		ProviderPrettyName: prettyName,
		AuthenticationMethods: []kubebindv1alpha2.AuthenticationMethod{
			{
				Method: "OAuth2CodeGrant",
				OAuth2CodeGrant: &kubebindv1alpha2.OAuth2CodeGrant{
					AuthenticatedURL: oidcAuthorizeURL,
				},
			},
		},
	}

	bs, err := json.Marshal(provider)
	if err != nil {
		logger.Error(err, "failed to marshal provider")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(bs) //nolint:errcheck
}

func (h *handler) handleTemplates(w http.ResponseWriter, r *http.Request) {
	logger := getLogger(r)
	prepareNoCache(w)
	logger.Info("getting templates")

	authenticated := auth.GetAuthContext(r.Context()).IsValid
	if !authenticated {
		logger.Info("unauthenticated request to get templates")
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	params := client.GetQueryParams(r)

	templates, err := h.listTemplates(r.Context(), params.ClusterID)
	if err != nil {
		logger.Error(err, "failed to get template resources")
		http.Error(w, "internal error", http.StatusInternalServerError)

		return
	}

	w.Header().Set("Content-Type", "application/json")
	// For UI, return direct result as before
	if err := json.NewEncoder(w).Encode(templates); err != nil {
		logger.Error(err, "failed to encode JSON response")
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

func (h *handler) handleCollections(w http.ResponseWriter, r *http.Request) {
	logger := getLogger(r)
	prepareNoCache(w)
	logger.Info("getting collections")

	params := client.GetQueryParams(r)

	collections, err := h.listCollections(r.Context(), params.ClusterID)
	if err != nil {
		logger.Error(err, "failed to get collection resources")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	// For UI, return direct result as before
	if err := json.NewEncoder(w).Encode(collections); err != nil {
		logger.Error(err, "failed to encode JSON response")
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

func (h *handler) handleBind(w http.ResponseWriter, r *http.Request) {
	logger := getLogger(r)
	params := client.GetQueryParams(r)

	prepareNoCache(w)

	// Get auth context (already verified by middleware)
	authCtx := auth.GetAuthContext(r.Context())
	state := authCtx.SessionState

	kfg, err := h.kubeManager.HandleResources(r.Context(), state.Token.Subject+"#"+state.ClusterID, params.ClusterID)
	if err != nil {
		logger.Error(err, "failed to handle resources")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Parse JSON request body
	const maxBodySize = 1 << 20 // 1 MB
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)
	var bindRequest kubebindv1alpha2.BindableResourcesRequest
	if err := json.NewDecoder(r.Body).Decode(&bindRequest); err != nil {
		logger.Error(err, "failed to parse JSON request body")
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
		} else {
			http.Error(w, "invalid JSON request body", http.StatusBadRequest)
		}
		return
	}

	// Module consist of many resources and permissionClaims. Read it and translate to
	template, err := h.kubeManager.GetTemplates(r.Context(), params.ClusterID, bindRequest.TemplateRef.Name)
	if err != nil {
		logger.Error(err, "failed to get template", "template", bindRequest.TemplateRef.Name, "cluster", params.ClusterID)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	request := kubebindv1alpha2.APIServiceExportRequestResponse{
		TypeMeta: metav1.TypeMeta{
			APIVersion: kubebindv1alpha2.SchemeGroupVersion.String(),
			Kind:       "APIServiceExportRequest",
		},
		ObjectMeta: kubebindv1alpha2.NameObjectMeta{
			Name: bindRequest.Name,
		},
		Spec: kubebindv1alpha2.APIServiceExportRequestSpec{
			Resources:        template.Spec.Resources,
			PermissionClaims: template.Spec.PermissionClaims,
			Namespaces:       template.Spec.Namespaces,
		},
	}

	// callback response
	requestBytes, err := json.Marshal(&request)
	if err != nil {
		logger.Error(err, "failed to marshal request")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	response := kubebindv1alpha2.BindingResourceResponse{
		TypeMeta: metav1.TypeMeta{
			APIVersion: kubebindv1alpha2.SchemeGroupVersion.String(),
			Kind:       "BindingResponse",
		},
		Authentication: kubebindv1alpha2.BindingResponseAuthentication{
			OAuth2CodeGrant: &kubebindv1alpha2.BindingResponseAuthenticationOAuth2CodeGrant{
				SessionID: state.SessionID,
				ID:        state.Token.Issuer + "/" + state.Token.Subject,
			},
		},
		Kubeconfig: kfg,
		Requests:   []runtime.RawExtension{{Raw: requestBytes}},
	}

	payload, err := json.Marshal(&response)
	if err != nil {
		logger.Error(err, "failed to marshal binding response")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(payload) //nolint:errcheck
}

// listTemplates fetches the list of APIServiceExportTemplates from the backend cluster without checking
// if they are part of a Collection or not.
func (h *handler) listTemplates(ctx context.Context, cluster string) (*kubebindv1alpha2.APIServiceExportTemplateList, error) {
	templates, err := h.kubeManager.ListTemplates(ctx, cluster)
	if err != nil {
		return nil, fmt.Errorf("failed to list collections: %w", err)
	}

	return templates, nil
}

// listCollections fetches the list of Collections from the backend cluster.
func (h *handler) listCollections(ctx context.Context, cluster string) (*kubebindv1alpha2.CollectionList, error) {
	collections, err := h.kubeManager.ListCollections(ctx, cluster)
	if err != nil {
		return nil, fmt.Errorf("failed to list collections: %w", err)
	}

	return collections, nil
}

func (h *handler) handleBindableResources(w http.ResponseWriter, r *http.Request) {
	logger := getLogger(r)

	bs, err := json.Marshal(&kubebindv1alpha2.ClaimableAPIs)
	if err != nil {
		logger.Error(err, "failed to marshal resources")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(bs) //nolint:errcheck
}

// prepareNoCache prepares headers for preventing browser caching.
func prepareNoCache(w http.ResponseWriter) {
	// Set NoCache headers
	for k, v := range noCacheHeaders {
		w.Header().Set(k, v)
	}
}

func getLogger(r *http.Request) klog.Logger {
	return klog.FromContext(r.Context()).WithValues("method", r.Method, "url", r.URL.String())
}
