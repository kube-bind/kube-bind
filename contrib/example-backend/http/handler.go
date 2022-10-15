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

	echo2 "github.com/labstack/echo/v4"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionslisters "k8s.io/apiextensions-apiserver/pkg/client/listers/apiextensions/v1"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/kube-bind/kube-bind/contrib/example-backend/cookie"
	"github.com/kube-bind/kube-bind/contrib/example-backend/kubernetes"
	"github.com/kube-bind/kube-bind/contrib/example-backend/kubernetes/resources"
	"github.com/kube-bind/kube-bind/contrib/example-backend/template"
	"github.com/kube-bind/kube-bind/pkg/apis/kubebind/v1alpha1"
)

var (
	resourcesTemplate = htmltemplate.Must(htmltemplate.New("resource").Parse(mustRead(template.Files.ReadFile, "resources.gohtml")))
)

type handler struct {
	oidc *oidcServiceProvider

	backendCallbackURL string
	providerPrettyName string

	client              *http.Client
	apiextensionsLister apiextensionslisters.CustomResourceDefinitionLister

	kubeManager *kubernetes.Manager
}

func NewHandler(
	provider *oidcServiceProvider,
	backendCallbackURL, providerPrettyName string,
	mgr *kubernetes.Manager,
	apiextensionsLister apiextensionslisters.CustomResourceDefinitionLister,
) (*handler, error) {
	return &handler{
		oidc:                provider,
		backendCallbackURL:  backendCallbackURL,
		providerPrettyName:  providerPrettyName,
		client:              http.DefaultClient,
		kubeManager:         mgr,
		apiextensionsLister: apiextensionsLister,
	}, nil
}

func (h *handler) handleServiceExport(c echo2.Context) error {
	serviceProvider := &v1alpha1.APIServiceProvider{
		Spec: v1alpha1.APIServiceProviderSpec{
			AuthenticatedClientURL: fmt.Sprintf("http://%s/authorize", c.Request().Host), // TODO: support https
			ProviderPrettyName:     h.providerPrettyName,
		},
	}

	bs, err := json.Marshal(serviceProvider)
	if err != nil {
		c.Logger().Error(err)
		return err
	}

	return c.Blob(http.StatusOK, "application/json", bs)
}

func (h *handler) handleAuthorize(c echo2.Context) error {
	scopes := []string{"openid", "profile", "email", "offline_access"}
	code := &resources.AuthCode{
		RedirectURL: c.QueryParam("u"),
		SessionID:   c.QueryParam("s"),
	}
	if code.RedirectURL == "" || code.SessionID == "" {
		http.Error(c.Response(), "missing redirect_url or session_id", http.StatusBadRequest)
		return nil
	}

	dataCode, err := json.Marshal(code)
	if err != nil {
		c.Logger().Error(err)
		return err
	}

	encoded := base64.StdEncoding.EncodeToString(dataCode)
	authURL := h.oidc.OIDCProviderConfig(scopes).AuthCodeURL(encoded)
	return c.Redirect(http.StatusSeeOther, authURL)
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
func (h *handler) handleCallback(c echo2.Context) error {
	if errMsg := c.FormValue("error"); errMsg != "" {
		http.Error(c.Response(), errMsg+": "+c.FormValue("error_description"), http.StatusBadRequest)
		return errors.New(errMsg)
	}
	code := c.FormValue("code")
	if code == "" {
		http.Error(c.Response(), fmt.Sprintf("no code in request: %q", c.Request().Form), http.StatusBadRequest)
		return nil
	}

	state := c.FormValue("state")
	decode, err := base64.StdEncoding.DecodeString(state)
	if err != nil {
		c.Logger().Error(err)
		return err
	}

	authCode := &resources.AuthCode{}
	if err := json.Unmarshal(decode, authCode); err != nil {
		c.Logger().Error(err)
		return err
	}

	// TODO: sign state and verify that it is not faked by the oauth provider

	token, err := h.oidc.OIDCProviderConfig(nil).Exchange(c.Request().Context(), code)
	if err != nil {
		http.Error(c.Response(), "internal error", http.StatusInternalServerError)
		c.Logger().Error(err)
		return err
	}
	jwtStr, ok := token.Extra("id_token").(string)
	if !ok {
		http.Error(c.Response(), "internal error", http.StatusInternalServerError)
		err := fmt.Errorf("invalid id token: %v", token.Extra("id_token"))
		c.Logger().Error(err)
		return err
	}

	jwt, err := parseJWT(jwtStr)
	if err != nil {
		http.Error(c.Response(), "internal error", http.StatusInternalServerError)
		c.Logger().Error(err)
		return err
	}
	if !ok {
		http.Error(c.Response(), "internal error", http.StatusInternalServerError)
		err := fmt.Errorf("invalid id token: %v", token.Extra("id_token"))
		c.Logger().Error(err)
		return err
	}

	sessionCookie := cookie.SessionState{
		CreatedAt:    time.Now(),
		ExpiresOn:    token.Expiry,
		AccessToken:  token.AccessToken,
		IDToken:      string(jwt),
		RefreshToken: token.RefreshToken,
		RedirectURL:  authCode.RedirectURL,
		SessionID:    authCode.SessionID,
	}

	b, err := sessionCookie.Encode()
	if err != nil {
		http.Error(c.Response(), "internal error", http.StatusInternalServerError)
		c.Logger().Error(fmt.Errorf("invalid id token: %v", jwt))
		return err
	}

	c.SetCookie(cookie.MakeCookie(
		c.Request(),
		"kube-bind-"+authCode.SessionID,
		b,
		time.Duration(1)*time.Hour),
	)

	return c.Redirect(302, "/resources?s="+authCode.SessionID)
}

func (h *handler) handleResources(c echo2.Context) error {
	crds, err := h.apiextensionsLister.List(labels.Everything())
	if err != nil {
		c.Logger().Error(err)
		return err
	}
	sort.SliceStable(crds, func(i, j int) bool {
		return crds[i].Name < crds[j].Name
	})

	bs := bytes.Buffer{}
	if err := resourcesTemplate.Execute(&bs, struct {
		SessionID string
		CRDs      []*apiextensionsv1.CustomResourceDefinition
	}{
		SessionID: c.Request().URL.Query().Get("s"),
		CRDs:      crds,
	}); err != nil {
		c.Logger().Error(err)
		return err
	}

	c.HTMLBlob(http.StatusOK, bs.Bytes()) // nolint: errcheck

	return nil
}

func (h *handler) handleBind(c echo2.Context) error {
	ck, err := c.Cookie("kube-bind-" + c.Request().URL.Query().Get("s"))
	if err != nil {
		c.Logger().Error(err)
		return err
	}

	state, err := cookie.Decode(ck.Value)
	if err != nil {
		c.Logger().Error(err)
		return err
	}

	var idToken struct {
		Subject string `json:"sub"`
		Issuer  string `json:"iss"`
	}
	if err := json.Unmarshal([]byte(state.IDToken), &idToken); err != nil {
		http.Error(c.Response(), "internal error", http.StatusInternalServerError)
		c.Logger().Error(fmt.Errorf("invalid id token: %v", state.IDToken))
		return err
	}

	group := c.Request().URL.Query().Get("group")
	resource := c.Request().URL.Query().Get("resource")
	kfg, err := h.kubeManager.HandleResources(c.Request().Context(), idToken.Subject, resource, group)
	if err != nil {
		c.Logger().Error(err)
		return err
	}

	// callback client with access token and kubeconfig
	authResponse := resources.AuthResponse{
		SessionID:  state.SessionID,
		ID:         idToken.Issuer + "/" + idToken.Subject,
		Kubeconfig: kfg,
		Group:      group,
		Resource:   resource,
		Export:     resource + "." + group,
	}

	payload, err := json.Marshal(authResponse)
	if err != nil {
		c.Logger().Error(err)
		return err
	}

	encoded := base64.StdEncoding.EncodeToString(payload)

	parsedAuthURL, err := url.Parse(state.RedirectURL)
	if err != nil {
		return fmt.Errorf("failed to parse auth url: %v", err)
	}

	values := parsedAuthURL.Query()
	values.Add("auth_response", encoded)

	parsedAuthURL.RawQuery = values.Encode()

	if err := c.Redirect(301, parsedAuthURL.String()); err != nil {
		c.Logger().Error(err)
		return err
	}

	return nil
}

func mustRead(f func(name string) ([]byte, error), name string) string {
	bs, err := f(name)
	if err != nil {
		panic(err)
	}
	return string(bs)
}
