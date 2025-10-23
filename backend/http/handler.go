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
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	htmltemplate "html/template"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/securecookie"
	"golang.org/x/oauth2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	componentbaseversion "k8s.io/component-base/version"
	"k8s.io/klog/v2"

	"github.com/kube-bind/kube-bind/backend/kubernetes"
	"github.com/kube-bind/kube-bind/backend/session"
	"github.com/kube-bind/kube-bind/backend/template"
	bindversion "github.com/kube-bind/kube-bind/pkg/version"
	kubebindv1alpha2 "github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2"
)

var (
	resourcesTemplate = htmltemplate.Must(htmltemplate.New("resource").Parse(mustRead(template.Files.ReadFile, "resources.gohtml")))
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
}

func NewHandler(
	provider *OIDCServiceProvider,
	oidcAuthorizeURL, backendCallbackURL, providerPrettyName, testingAutoSelect string,
	cookieSigningKey, cookieEncryptionKey []byte,
	schemaSource string,
	scope kubebindv1alpha2.InformerScope,
	mgr *kubernetes.Manager,
) (*handler, error) {
	return &handler{
		oidc:                provider,
		oidcAuthorizeURL:    oidcAuthorizeURL,
		backendCallbackURL:  backendCallbackURL,
		providerPrettyName:  providerPrettyName,
		testingAutoSelect:   testingAutoSelect,
		schemaSource:        schemaSource,
		scope:               scope,
		client:              http.DefaultClient,
		kubeManager:         mgr,
		cookieSigningKey:    cookieSigningKey,
		cookieEncryptionKey: cookieEncryptionKey,
	}, nil
}

func (h *handler) AddRoutes(mux *mux.Router) {
	// Server contains double routes for when backend is multi-cluster aware or single cluster.
	// When called multi-cluster aware route in single cluster mode, it will ignore cluster parameter.
	mux.HandleFunc("/clusters/{cluster}/exports", h.handleServiceExport).Methods("GET")
	mux.HandleFunc("/exports", h.handleServiceExport).Methods("GET")

	mux.HandleFunc("/clusters/{cluster}/resources", h.handleResources).Methods("GET")
	mux.HandleFunc("/resources", h.handleResources).Methods("GET")

	mux.HandleFunc("/clusters/{cluster}/bind", h.handleBind).Methods("GET")
	mux.HandleFunc("/bind", h.handleBind).Methods("GET")

	mux.HandleFunc("/clusters/{cluster}/authorize", h.handleAuthorize).Methods("GET")
	mux.HandleFunc("/authorize", h.handleAuthorize).Methods("GET")

	mux.HandleFunc("/callback", h.handleCallback).Methods("GET")
	mux.HandleFunc("/healthz", h.handleHealthz).Methods("GET")
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
			oidcAuthorizeURL = fmt.Sprintf("http://%s/authorize", r.Host)
		} else {
			oidcAuthorizeURL = fmt.Sprintf("http://%s/clusters/%s/authorize", r.Host, cluster)
		}
	}

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
		code.RedirectURL = fmt.Sprintf("http://127.0.0.1:%s/callback", callbackPort)
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
	Scope string // "Namespaced" or "Cluster"

	Name        string
	Description string

	Resources        []kubebindv1alpha2.APIServiceExportResource
	PermissionClaims []kubebindv1alpha2.PermissionClaim
	Namespaces       []kubebindv1alpha2.Namespaces

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

	templates, err := h.listCollectionTemplates(r.Context(), providerCluster)
	if err != nil {
		logger.Error(err, "failed to get template resources")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	result := make([]UISchema, 0, len(templates.Items))
	for _, item := range templates.Items {
		if !strings.EqualFold(h.scope.String(), string(item.Spec.Scope)) && h.scope != kubebindv1alpha2.ClusterScope {
			continue
		}

		result = append(result, UISchema{
			Name:             item.GetName(),
			Scope:            string(item.Spec.Scope),
			PermissionClaims: item.Spec.PermissionClaims,
			Resources:        item.Spec.Resources,
			Namespaces:       item.Spec.Namespaces,
			SessionID:        sessionID,
		})
	}

	bs := bytes.Buffer{}
	if err := resourcesTemplate.Execute(&bs, struct {
		Cluster string
		Schemas []UISchema
	}{
		Cluster: providerCluster,
		Schemas: result,
	}); err != nil {
		logger.Error(err, "failed to execute template")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(bs.Bytes()) //nolint:errcheck
}

func (h *handler) handleBind(w http.ResponseWriter, r *http.Request) {
	logger := getLogger(r)
	templateName := r.URL.Query().Get("template")
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

	// Module consist of many resources and permissionClaims. Read it and translate to
	template, err := h.kubeManager.GetTemplates(r.Context(), providerCluster, templateName)
	if err != nil {
		logger.Error(err, "failed to get template")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	request := kubebindv1alpha2.APIServiceExportRequestResponse{
		TypeMeta: metav1.TypeMeta{
			APIVersion: kubebindv1alpha2.SchemeGroupVersion.String(),
			Kind:       "APIServiceExportRequest",
		},
		ObjectMeta: kubebindv1alpha2.NameObjectMeta{
			Name: templateName,
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

func mustRead(f func(name string) ([]byte, error), name string) string {
	bs, err := f(name)
	if err != nil {
		panic(err)
	}
	return string(bs)
}

// listCollectionTemplates fetches the list of Collections from the backend cluster.
// Flow is:
// 1. List Collection and check what modules we are targeting
// 2. Get templates from the backend cluster and construct shallow-bound schemas (no crd content).
func (h *handler) listCollectionTemplates(ctx context.Context, cluster string) (*kubebindv1alpha2.APIServiceExportTemplateList, error) {
	collections, err := h.kubeManager.ListCollections(ctx, cluster)
	if err != nil {
		return nil, fmt.Errorf("failed to list collections: %w", err)
	}

	templates := &kubebindv1alpha2.APIServiceExportTemplateList{}
	for _, collection := range collections.Items {
		for _, t := range collection.Spec.Templates {
			template, err := h.kubeManager.GetTemplates(ctx, cluster, t.Name)
			if err != nil {
				return nil, fmt.Errorf("failed to get template %q: %w", t.Name, err)
			}
			templates.Items = append(templates.Items, *template)
		}
	}

	return templates, nil
}
