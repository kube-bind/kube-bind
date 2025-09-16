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
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/securecookie"
	"golang.org/x/oauth2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	componentbaseversion "k8s.io/component-base/version"
	"k8s.io/klog/v2"

	"github.com/kube-bind/kube-bind/backend/kubernetes"
	"github.com/kube-bind/kube-bind/backend/kubernetes/resources"
	"github.com/kube-bind/kube-bind/backend/session"
	"github.com/kube-bind/kube-bind/backend/spaserver"
	bindversion "github.com/kube-bind/kube-bind/pkg/version"
	kubebindv1alpha2 "github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2"
)

// See https://developers.google.com/web/fundamentals/performance/optimizing-content-efficiency/http-caching?hl=en
var noCacheHeaders = map[string]string{
	"Expires":         time.Unix(0, 0).Format(time.RFC1123),
	"Cache-Control":   "no-cache, no-store, must-revalidate, max-age=0",
	"X-Accel-Expires": "0", // https://www.nginx.com/resources/wiki/start/topics/examples/x-accel/
}

type handler struct {
	oidc *OIDCServiceProvider

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

	frontend string
}

func NewHandler(
	provider *OIDCServiceProvider,
	oidcAuthorizeURL, backendCallbackURL, providerPrettyName, testingAutoSelect string,
	cookieSigningKey, cookieEncryptionKey []byte,
	schemaSource string,
	scope kubebindv1alpha2.InformerScope,
	mgr *kubernetes.Manager,
	frontend string,
) (*handler, error) {
	return &handler{
		oidc:                provider,
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
	}, nil
}

func (h *handler) AddRoutes(mux *mux.Router) {
	// API routes - Server contains double routes for when backend is multi-cluster aware or single cluster.
	// When called multi-cluster aware route in single cluster mode, it will ignore cluster parameter.
	mux.HandleFunc("/api/clusters/{cluster}/exports", h.handleServiceExport).Methods(http.MethodGet)
	mux.HandleFunc("/api/exports", h.handleServiceExport).Methods(http.MethodGet)

	mux.HandleFunc("/api/clusters/{cluster}/resources", h.handleResources).Methods(http.MethodGet)
	mux.HandleFunc("/api/resources", h.handleResources).Methods(http.MethodGet)

	mux.HandleFunc("/api/clusters/{cluster}/bind", h.handleBind).Methods(http.MethodGet)
	mux.HandleFunc("/api/bind", h.handleBind).Methods(http.MethodGet)

	mux.HandleFunc("/api/clusters/{cluster}/bind", h.handleBindPost).Methods(http.MethodPost)
	mux.HandleFunc("/api/bind", h.handleBindPost).Methods(http.MethodPost)

	mux.HandleFunc("/api/clusters/{cluster}/authorize", h.handleAuthorize).Methods(http.MethodGet)
	mux.HandleFunc("/api/authorize", h.handleAuthorize).Methods(http.MethodGet)

	mux.HandleFunc("/api/callback", h.handleCallback).Methods(http.MethodGet)
	mux.HandleFunc("/api/healthz", h.handleHealthz).Methods(http.MethodGet)

	if strings.HasPrefix(h.frontend, "http://") {
		spaserver, err := spaserver.NewSPAReverseProxyServer(h.frontend)
		if err != nil {
			panic(fmt.Sprintf("failed to create SPA reverse proxy server: %v", err)) // Development only.
		}
		mux.PathPrefix("/").Handler(spaserver)
	} else {
		fileSystem := http.Dir(h.frontend)
		mux.PathPrefix("/").Handler(spaserver.NewSPAFileServer(fileSystem))
	}

}

func (h *handler) handleHealthz(w http.ResponseWriter, r *http.Request) {
	prepareNoCache(w)
	w.WriteHeader(http.StatusOK)
}

func (h *handler) handleServiceExport(w http.ResponseWriter, r *http.Request) {
	logger := klog.FromContext(r.Context()).WithValues("method", r.Method, "url", r.URL.String())
	cluster := mux.Vars(r)["cluster"]
	singleClusterScoped := cluster == ""

	oidcAuthorizeURL := h.oidcAuthorizeURL
	if oidcAuthorizeURL == "" {
		if singleClusterScoped {
			oidcAuthorizeURL = fmt.Sprintf("http://%s/api/authorize", r.Host)
		} else {
			oidcAuthorizeURL = fmt.Sprintf("http://%s/api/clusters/%s/authorize", r.Host, cluster)
		}
	}

	ver, err := bindversion.BinaryVersion(componentbaseversion.Get().GitVersion)
	if err != nil {
		logger.Error(err, "failed to parse version %q", componentbaseversion.Get().GitVersion)
		ver = "v0.0.0"
	}

	provider := &kubebindv1alpha2.BindingProvider{
		TypeMeta: metav1.TypeMeta{
			APIVersion: kubebindv1alpha2.GroupVersion,
			Kind:       "BindingProvider",
		},
		Version:            ver,
		ProviderPrettyName: "backend",
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

func generateCookieName(clusterID string) string {
	if clusterID == "" {
		return "kube-bind"
	}

	return fmt.Sprintf("kube-bind-%s", clusterID)
}

func (h *handler) handleAuthorize(w http.ResponseWriter, r *http.Request) {
	logger := getLogger(r)

	providerCluster := mux.Vars(r)["cluster"]

	callbackPort := r.URL.Query().Get("p")

	scopes := []string{"openid", "profile", "email", "offline_access"}
	code := &AuthCode{
		RedirectURL:       r.URL.Query().Get("u"),
		SessionID:         r.URL.Query().Get("s"),
		ClusterID:         r.URL.Query().Get("c"),
		ProviderClusterID: providerCluster, // used in multicluster-runtime providers
	}
	if callbackPort != "" && code.RedirectURL == "" {
		code.RedirectURL = fmt.Sprintf("http://localhost:%s/api/callback", callbackPort)
	}

	if code.RedirectURL == "" || code.SessionID == "" || code.ClusterID == "" {
		logger.Error(errors.New("missing redirect url or session id or cluster id"), "failed to authorize")
		http.Error(w, "missing redirect_url, session_id or cluster_id", http.StatusBadRequest)
		return
	}

	dataCode, err := json.Marshal(code)
	if err != nil {
		logger.Info("failed to marshal auth code", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	encoded := base64.URLEncoding.EncodeToString(dataCode)
	authURL := h.oidc.OIDCProviderConfig(scopes).AuthCodeURL(encoded)
	http.Redirect(w, r, authURL, http.StatusFound)
}

// handleCallback handle the authorization redirect callback from OAuth2 auth flow.
func (h *handler) handleCallback(w http.ResponseWriter, r *http.Request) {
	logger := getLogger(r)

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
	decoded, err := base64.StdEncoding.DecodeString(state)
	if err != nil {
		logger.Error(err, "failed to decode state")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	authCode := &AuthCode{}
	if err := json.Unmarshal(decoded, authCode); err != nil {
		logger.Error(err, "failed to unmarshal authCode")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// TODO: sign state and verify that it is not faked by the oauth provider

	token, err := h.oidc.OIDCProviderConfig(nil).Exchange(r.Context(), code)
	if err != nil {
		logger.Error(err, "failed to exchange token")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	cookieName := generateCookieName(authCode.ClusterID)
	sessionState, err := createSessionState(authCode, token)
	if err != nil {
		logger.Error(err, "failed to create session sessionState")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	s := securecookie.New(h.cookieSigningKey, h.cookieEncryptionKey)
	encoded, err := s.Encode(cookieName, sessionState)
	if err != nil {
		logger.Error(err, "failed to encode secure session cookie")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// setting to false so it works over http://localhost
	secure := false

	http.SetCookie(w, session.MakeCookie(r, cookieName, encoded, secure, 1*time.Hour))
	if authCode.ProviderClusterID == "" {
		http.Redirect(w, r, "/resources?s="+cookieName, http.StatusFound)
	} else {
		http.Redirect(w, r, "/clusters/"+authCode.ProviderClusterID+"/resources?s="+cookieName, http.StatusFound)
	}
}

func unwrapJWT(p string) ([]byte, error) {
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

func createSessionState(authCode *AuthCode, token *oauth2.Token) (*session.State, error) {
	jwtStr, ok := token.Extra("id_token").(string)
	if !ok {
		return nil, errors.New("no id_token value found in token")
	}

	jwt, err := unwrapJWT(jwtStr)
	if err != nil {
		return nil, fmt.Errorf("failed to unpack ID token: %w", err)
	}

	var idToken struct {
		Subject string `json:"sub"`
		Issuer  string `json:"iss"`
	}
	if err := json.Unmarshal(jwt, &idToken); err != nil {
		return nil, fmt.Errorf("failed to parse ID token: %w", err)
	}

	return &session.State{
		Token: session.TokenInfo{
			Subject: idToken.Subject,
			Issuer:  idToken.Issuer,
		},
		SessionID:   authCode.SessionID,
		ClusterID:   authCode.ClusterID,
		RedirectURL: authCode.RedirectURL,
	}, nil
}

type UISchema struct {
	Name     string
	Version  string
	Group    string
	Kind     string
	Scope    string // "Namespaced" or "Cluster"
	Resource string

	// SessionID
	SessionID string
}

func (h *handler) handleResources(w http.ResponseWriter, r *http.Request) {
	logger := getLogger(r)

	prepareNoCache(w)

	providerCluster := mux.Vars(r)["cluster"]
	sessionID := r.URL.Query().Get("s")
	singleClusterScoped := providerCluster == ""

	if h.testingAutoSelect != "" {
		parts := strings.SplitN(h.testingAutoSelect, ".", 2)
		if singleClusterScoped {
			http.Redirect(w, r, "/resources/"+parts[0]+"/"+parts[1], http.StatusFound)
		} else {
			http.Redirect(w, r, "/clusters/"+providerCluster+"/resources/"+parts[0]+"/"+parts[1], http.StatusFound)
		}
		return
	}

	exportedSchemas, err := h.getBackendDynamicResource(r.Context(), providerCluster)
	if err != nil {
		logger.Error(err, "failed to get dynamic resources")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	result := kubebindv1alpha2.BindableResourcesResponse{}
	for _, item := range exportedSchemas {

		result.Resources = append(result.Resources, kubebindv1alpha2.BindableResource{
			Name:       item.GetName(),
			Kind:       item.Spec.Names.Kind,
			Scope:      string(item.Spec.Scope),
			APIVersion: item.Spec.Versions[0].Name,
			Group:      item.Spec.Group,
			// Important: This MUST be used as UI button class in the url, so tests can 'click it' based on it.
			Resource:  item.Spec.Names.Plural,
			SessionID: sessionID,
		})
	}

	bs, err := json.Marshal(&result)
	if err != nil {
		logger.Error(err, "failed to marshal resources")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(bs) //nolint:errcheck

}

func (h *handler) handleBind(w http.ResponseWriter, r *http.Request) {
	logger := getLogger(r)
	group := r.URL.Query().Get("group")
	resource := r.URL.Query().Get("resource")
	version := r.URL.Query().Get("version")
	providerCluster := mux.Vars(r)["cluster"]

	prepareNoCache(w)

	cookieName := r.URL.Query().Get("s")
	ck, err := r.Cookie(r.URL.Query().Get("s"))
	if err != nil {
		logger.Error(err, "failed to get session cookie")
		http.Error(w, "no session cookie found", http.StatusBadRequest)
		return
	}

	state := session.State{}
	s := securecookie.New(h.cookieSigningKey, h.cookieEncryptionKey)
	if err := s.Decode(cookieName, ck.Value, &state); err != nil {
		logger.Error(err, "failed to decode session cookie")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	kfg, err := h.kubeManager.HandleResources(r.Context(), state.Token.Subject+"#"+state.ClusterID, providerCluster)
	if err != nil {
		logger.Error(err, "failed to handle resources")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	request := kubebindv1alpha2.APIServiceExportRequestResponse{
		TypeMeta: metav1.TypeMeta{
			APIVersion: kubebindv1alpha2.SchemeGroupVersion.String(),
			Kind:       "APIServiceExportRequest",
		},
		ObjectMeta: kubebindv1alpha2.NameObjectMeta{
			// this is good for one resource. If there are more (in the future),
			// we need a better name heuristic. Note: it does not have to be unique.
			// But pretty is better.
			Name: resource + "." + group,
		},
		Spec: kubebindv1alpha2.APIServiceExportRequestSpec{
			Resources: []kubebindv1alpha2.APIServiceExportRequestResource{
				{
					GroupResource: kubebindv1alpha2.GroupResource{Group: group, Resource: resource},
					Versions:      []string{version},
				},
			},
		},
	}

	// callback response
	requestBytes, err := json.Marshal(&request)
	if err != nil {
		logger.Error(err, "failed to marshal request")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	response := kubebindv1alpha2.BindingResponse{
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

	encoded := base64.URLEncoding.EncodeToString(payload)

	parsedAuthURL, err := url.Parse(state.RedirectURL)
	if err != nil {
		logger.Error(err, "failed to parse redirect URL")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	values := parsedAuthURL.Query()
	values.Add("response", encoded)

	parsedAuthURL.RawQuery = values.Encode()

	logger.V(1).Info("redirecting to auth callback", "url", state.RedirectURL+"?response=<redacted>")
	http.Redirect(w, r, parsedAuthURL.String(), http.StatusFound)
}

func (h *handler) handleBindPost(w http.ResponseWriter, r *http.Request) {
	logger := getLogger(r)
	providerCluster := mux.Vars(r)["cluster"]

	prepareNoCache(w)

	// Get session ID from query parameter
	sessionID := r.URL.Query().Get("s")
	if sessionID == "" {
		logger.Error(errors.New("missing session parameter"), "failed to get session from query")
		http.Error(w, "missing session parameter 's'", http.StatusBadRequest)
		return
	}

	// Get session cookie
	ck, err := r.Cookie(sessionID)
	if err != nil {
		logger.Error(err, "failed to get session cookie")
		http.Error(w, "no session cookie found", http.StatusBadRequest)
		return
	}

	// Decode session state
	state := session.State{}
	s := securecookie.New(h.cookieSigningKey, h.cookieEncryptionKey)
	if err := s.Decode(sessionID, ck.Value, &state); err != nil {
		logger.Error(err, "failed to decode session cookie")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Parse JSON request body
	var bindRequest kubebindv1alpha2.BindableResourcesRequest
	if err := json.NewDecoder(r.Body).Decode(&bindRequest); err != nil {
		logger.Error(err, "failed to parse JSON request body")
		http.Error(w, "invalid JSON request body", http.StatusBadRequest)
		return
	}

	logger.V(1).Info("received bind request", "resources", len(bindRequest.Resources), "permissionClaims", len(bindRequest.PermissionClaims))

	// Validate request
	if len(bindRequest.Resources) == 0 {
		logger.Error(errors.New("no resources specified"), "validation failed")
		http.Error(w, "at least one resource must be specified", http.StatusBadRequest)
		return
	}

	// Handle kubeconfig setup
	kfg, err := h.kubeManager.HandleResources(r.Context(), state.Token.Subject+"#"+state.ClusterID, providerCluster)
	if err != nil {
		logger.Error(err, "failed to handle resources")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Create API service export requests for each resource
	var requests []runtime.RawExtension
	for _, bindableResource := range bindRequest.Resources {
		exportRequest := kubebindv1alpha2.APIServiceExportRequestResponse{
			TypeMeta: metav1.TypeMeta{
				APIVersion: kubebindv1alpha2.SchemeGroupVersion.String(),
				Kind:       "APIServiceExportRequest",
			},
			ObjectMeta: kubebindv1alpha2.NameObjectMeta{
				Name: bindableResource.Resource + "." + bindableResource.Group,
			},
			Spec: kubebindv1alpha2.APIServiceExportRequestSpec{
				Resources: []kubebindv1alpha2.APIServiceExportRequestResource{
					{
						GroupResource: kubebindv1alpha2.GroupResource{
							Group:    bindableResource.Group,
							Resource: bindableResource.Resource,
						},
						Versions: []string{bindableResource.APIVersion},
					},
				},
			},
		}

		requestBytes, err := json.Marshal(&exportRequest)
		if err != nil {
			logger.Error(err, "failed to marshal export request", "resource", bindableResource.Resource)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		requests = append(requests, runtime.RawExtension{Raw: requestBytes})
	}

	// Create binding response
	response := kubebindv1alpha2.BindingResponse{
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
		Requests:   requests,
	}

	payload, err := json.Marshal(&response)
	if err != nil {
		logger.Error(err, "failed to marshal binding response")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	encoded := base64.URLEncoding.EncodeToString(payload)

	parsedAuthURL, err := url.Parse(state.RedirectURL)
	if err != nil {
		logger.Error(err, "failed to parse redirect URL")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	values := parsedAuthURL.Query()
	values.Add("response", encoded)

	parsedAuthURL.RawQuery = values.Encode()

	logger.V(1).Info("redirecting to auth callback after POST bind",
		"url", state.RedirectURL+"?response=<redacted>",
		"resourceCount", len(bindRequest.Resources))

	// For POST requests, we can either redirect or return JSON response
	// Since this is called from frontend JavaScript, return the redirect URL as JSON
	w.Header().Set("Content-Type", "application/json")
	redirectResponse := map[string]string{
		"redirectURL": parsedAuthURL.String(),
	}

	if err := json.NewEncoder(w).Encode(redirectResponse); err != nil {
		logger.Error(err, "failed to encode redirect response")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
}

func (h *handler) getBackendDynamicResource(ctx context.Context, cluster string) (kubebindv1alpha2.ExportedSchemas, error) {
	labelSelector := labels.Set{
		resources.ExportedCRDsLabel: "true",
	}

	parts := strings.SplitN(h.schemaSource, ".", 3)
	if len(parts) != 3 { // We check this in validation, but just in case.
		return nil, fmt.Errorf("invalid schema source: %q", h.schemaSource)
	}

	gvk := schema.GroupVersionKind{
		Kind:    parts[0],
		Version: parts[1],
		Group:   parts[2],
	}
	return h.kubeManager.ListDynamicResources(ctx, cluster, gvk, labelSelector.AsSelector())
}
