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
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	htmltemplate "html/template"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/gorilla/mux"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionslisters "k8s.io/apiextensions-apiserver/pkg/client/listers/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"

	"github.com/kube-bind/kube-bind/contrib/example-backend/cookie"
	"github.com/kube-bind/kube-bind/contrib/example-backend/kubernetes"
	"github.com/kube-bind/kube-bind/contrib/example-backend/template"
	kubebindv1alpha1 "github.com/kube-bind/kube-bind/pkg/apis/kubebind/v1alpha1"
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

	backendCallbackURL string
	providerPrettyName string
	testingAutoSelect  string

	client              *http.Client
	apiextensionsLister apiextensionslisters.CustomResourceDefinitionLister

	kubeManager *kubernetes.Manager
}

func NewHandler(
	provider *OIDCServiceProvider,
	backendCallbackURL, providerPrettyName, testingAutoSelect string,
	mgr *kubernetes.Manager,
	apiextensionsLister apiextensionslisters.CustomResourceDefinitionLister,
) (*handler, error) {
	return &handler{
		oidc:                provider,
		backendCallbackURL:  backendCallbackURL,
		providerPrettyName:  providerPrettyName,
		testingAutoSelect:   testingAutoSelect,
		client:              http.DefaultClient,
		kubeManager:         mgr,
		apiextensionsLister: apiextensionsLister,
	}, nil
}

func (h *handler) AddRoutes(mux *mux.Router) {
	mux.HandleFunc("/export", h.handleServiceExport).Methods("GET")
	mux.HandleFunc("/resources", h.handleResources).Methods("GET")
	mux.HandleFunc("/bind", h.handleBind).Methods("GET")
	mux.HandleFunc("/authorize", h.handleAuthorize).Methods("GET")
	mux.HandleFunc("/callback", h.handleCallback).Methods("GET")
}

func (h *handler) handleServiceExport(w http.ResponseWriter, r *http.Request) {
	logger := klog.FromContext(r.Context()).WithValues("method", r.Method, "url", r.URL.String())

	provider := &kubebindv1alpha1.BindingProvider{
		TypeMeta: metav1.TypeMeta{
			APIVersion: kubebindv1alpha1.GroupVersion,
			Kind:       "BindingProvider",
		},
		ProviderPrettyName: "example-backend",
		AuthenticationMethods: []kubebindv1alpha1.AuthenticationMethod{
			{
				Method: "OAuth2CodeGrant",
				OAuth2CodeGrant: &kubebindv1alpha1.OAuth2CodeGrant{
					AuthenticatedURL: fmt.Sprintf("http://%s/authorize", r.Host),
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
	w.Write(bs) // nolint:errcheck
}

// prepareNoCache prepares headers for preventing browser caching.
func prepareNoCache(w http.ResponseWriter) {
	// Set NoCache headers
	for k, v := range noCacheHeaders {
		w.Header().Set(k, v)
	}
}

func (h *handler) handleAuthorize(w http.ResponseWriter, r *http.Request) {
	logger := klog.FromContext(r.Context()).WithValues("method", r.Method, "url", r.URL.String())

	scopes := []string{"openid", "profile", "email", "offline_access"}
	code := &AuthCode{
		RedirectURL: r.URL.Query().Get("u"),
		SessionID:   r.URL.Query().Get("s"),
		ClusterID:   r.URL.Query().Get("c"),
	}
	if code.RedirectURL == "" || code.SessionID == "" || code.ClusterID == "" {
		logger.Error(errors.New("missing redirect url or session id or cluster id"), "failed to authorize")
		http.Error(w, "missing redirect_url or session_id", http.StatusBadRequest)
		return
	}

	dataCode, err := json.Marshal(code)
	if err != nil {
		logger.Info("failed to marshal auth code", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	encoded := base64.StdEncoding.EncodeToString(dataCode)
	authURL := h.oidc.OIDCProviderConfig(scopes).AuthCodeURL(encoded)
	http.Redirect(w, r, authURL, http.StatusFound)
}

func parseJWT(p string) ([]byte, error) {
	parts := strings.Split(p, ".")
	if len(parts) < 2 {
		return nil, fmt.Errorf("oidc: malformed jwt, expected 3 parts got %d", len(parts))
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("oidc: malformed jwt payload: %v", err)
	}
	return payload, nil
}

// handleCallback handle the authorization redirect callback from OAuth2 auth flow.
func (h *handler) handleCallback(w http.ResponseWriter, r *http.Request) {
	logger := klog.FromContext(r.Context()).WithValues("method", r.Method, "url", r.URL.String())

	if errMsg := r.Form.Get("error"); errMsg != "" {
		logger.Info("failed to authorize", "error", errMsg)
		http.Error(w, errMsg+": "+r.Form.Get("error_description"), http.StatusBadRequest)
		return
	}
	code := r.Form.Get("code")
	if code == "" {
		code = r.URL.Query().Get("code")
	}
	if code == "" {
		logger.Info("no code in request", "error", "missing code")
		http.Error(w, fmt.Sprintf("no code in request: %q", r.Form), http.StatusBadRequest)
		return
	}

	state := r.Form.Get("state")
	if state == "" {
		state = r.URL.Query().Get("state")
	}
	decoded, err := base64.StdEncoding.DecodeString(state)
	if err != nil {
		logger.Info("failed to decode state", "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	authCode := &AuthCode{}
	if err := json.Unmarshal(decoded, authCode); err != nil {
		logger.Info("faile to unmarshal authCode", "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// TODO: sign state and verify that it is not faked by the oauth provider

	token, err := h.oidc.OIDCProviderConfig(nil).Exchange(r.Context(), code)
	if err != nil {
		logger.Info("failed to exchange token", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	jwtStr, ok := token.Extra("id_token").(string)
	if !ok {
		logger.Info("failed to get id_token from token", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	jwt, err := parseJWT(jwtStr)
	if err != nil {
		logger.Info("failed to parse jwt", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if !ok {
		logger.Info("failed to get id_token from token", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	sessionCookie := cookie.SessionState{
		CreatedAt:    time.Now(),
		ExpiresOn:    token.Expiry,
		AccessToken:  token.AccessToken,
		IDToken:      string(jwt),
		RefreshToken: token.RefreshToken,
		RedirectURL:  authCode.RedirectURL,
		SessionID:    authCode.SessionID,
		ClusterID:    authCode.ClusterID,
	}

	b, err := sessionCookie.Encode()
	if err != nil {
		logger.Info("failed to encode session cookie", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, cookie.MakeCookie(
		r,
		"kube-bind-"+authCode.SessionID,
		b,
		time.Duration(1)*time.Hour),
	)

	http.Redirect(w, r, "/resources?s="+authCode.SessionID, http.StatusFound)
}

func (h *handler) handleResources(w http.ResponseWriter, r *http.Request) {
	logger := klog.FromContext(r.Context()).WithValues("method", r.Method, "url", r.URL.String())

	prepareNoCache(w)

	if h.testingAutoSelect != "" {
		parts := strings.SplitN(h.testingAutoSelect, ".", 2)
		http.Redirect(w, r, "/resources/"+parts[0]+"/"+parts[1], http.StatusFound)
		return
	}

	crds, err := h.apiextensionsLister.List(labels.Everything())
	if err != nil {
		logger.Info("failed to list crds", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	sort.SliceStable(crds, func(i, j int) bool {
		return crds[i].Name < crds[j].Name
	})

	bs := bytes.Buffer{}
	if err := resourcesTemplate.Execute(&bs, struct {
		SessionID string
		CRDs      []*apiextensionsv1.CustomResourceDefinition
	}{
		SessionID: r.URL.Query().Get("s"),
		CRDs:      crds,
	}); err != nil {
		logger.Info("failed to execute template", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	w.Write(bs.Bytes()) // nolint:errcheck
}

func (h *handler) handleBind(w http.ResponseWriter, r *http.Request) {
	logger := klog.FromContext(r.Context()).WithValues("method", r.Method, "url", r.URL.String())

	prepareNoCache(w)

	ck, err := r.Cookie("kube-bind-" + r.URL.Query().Get("s"))
	if err != nil {
		logger.Info("failed to get session cookie", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	state, err := cookie.Decode(ck.Value)
	if err != nil {
		logger.Info("failed to decode session cookie", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	var idToken struct {
		Subject string `json:"sub"`
		Issuer  string `json:"iss"`
	}
	if err := json.Unmarshal([]byte(state.IDToken), &idToken); err != nil {
		logger.Info("failed to unmarshal id token", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	group := r.URL.Query().Get("group")
	resource := r.URL.Query().Get("resource")
	kfg, err := h.kubeManager.HandleResources(r.Context(), idToken.Subject+"#"+state.ClusterID, resource, group)
	if err != nil {
		logger.Info("failed to handle resources", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	request := kubebindv1alpha1.APIServiceBindingRequestResponse{
		TypeMeta: metav1.TypeMeta{
			APIVersion: kubebindv1alpha1.SchemeGroupVersion.String(),
			Kind:       "APIServiceBindingRequest",
		},
		ObjectMeta: kubebindv1alpha1.NameObjectMeta{
			// this is good for one resource. If there are more (in the future),
			// we need a better name heuristic. Note: it does not have to be unique.
			// But pretty is better.
			Name: resource + "." + group,
		},
		Spec: kubebindv1alpha1.APIServiceBindingRequestSpec{
			Resources: []kubebindv1alpha1.APIServiceBindingRequestResource{
				{GroupResource: kubebindv1alpha1.GroupResource{Group: group, Resource: resource}},
			},
		},
	}

	// callback response
	requestBytes, err := json.Marshal(&request)
	if err != nil {
		logger.Info("failed to marshal request", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	response := kubebindv1alpha1.BindingResponse{
		TypeMeta: metav1.TypeMeta{
			APIVersion: kubebindv1alpha1.SchemeGroupVersion.String(),
			Kind:       "BindingResponse",
		},
		Authentication: kubebindv1alpha1.BindingResponseAuthentication{
			OAuth2CodeGrant: &kubebindv1alpha1.BindingResponseAuthenticationOAuth2CodeGrant{
				SessionID: state.SessionID,
				ID:        idToken.Issuer + "/" + idToken.Subject,
			},
		},
		Kubeconfig: kfg,
		Requests:   []runtime.RawExtension{{Raw: requestBytes}},
	}
	payload, err := json.Marshal(&response)
	if err != nil {
		logger.Info("failed to marshal auth response", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	encoded := base64.StdEncoding.EncodeToString(payload)

	parsedAuthURL, err := url.Parse(state.RedirectURL)
	if err != nil {
		logger.Info("failed to parse redirect url", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	values := parsedAuthURL.Query()
	values.Add("response", encoded)

	parsedAuthURL.RawQuery = values.Encode()

	http.Redirect(w, r, parsedAuthURL.String(), http.StatusFound)
}

func mustRead(f func(name string) ([]byte, error), name string) string {
	bs, err := f(name)
	if err != nil {
		panic(err)
	}
	return string(bs)
}
