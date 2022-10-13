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

	echo2 "github.com/labstack/echo/v4"

	"github.com/kube-bind/kube-bind/contrib/example-backend/kubernetes"
	"github.com/kube-bind/kube-bind/contrib/example-backend/kubernetes/resources"
	"github.com/kube-bind/kube-bind/pkg/apis/kubebind/v1alpha1"
)

type handler struct {
	oidc *oidcServiceProvider

	client *http.Client

	kubeManager *kubernetes.Manager

	clusterName string
}

func NewHandler(provider *oidcServiceProvider, mgr *kubernetes.Manager, clusterName string) (*handler, error) {
	return &handler{
		oidc:        provider,
		client:      http.DefaultClient,
		kubeManager: mgr,
		clusterName: clusterName,
	}, nil
}

func (h *handler) handleServiceExport(c echo2.Context) error {
	serviceProvider := &v1alpha1.APIServiceProvider{
		Spec: v1alpha1.APIServiceProviderSpec{
			AuthenticatedClientURL: "http://ec2-18-196-160-160.eu-central-1.compute.amazonaws.com/authorize",
			ProviderPrettyName:     "MangoDB.Inc",
		},
	}

	blob, err := json.Marshal(serviceProvider)
	if err != nil {
		c.Logger().Error(err)
		return err
	}

	return c.Blob(http.StatusOK, "application/json", blob)
}

func (h *handler) handleAuthorize(c echo2.Context) error {
	var scopes = []string{"openid", "profile", "email", "offline_access"}
	code := &resources.AuthCode{}
	code.RedirectURL = c.QueryParam("redirect_url")
	code.SessionID = c.QueryParam("session_id")

	if err := h.kubeManager.SaveUserData(context.TODO(), code); err != nil {
		c.Logger().Error(err)
		return err
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

func (h *handler) handleCallback(c echo2.Context) error {
	// Authorization redirect callback from OAuth2 auth flow.
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
	sessionIDSecret, err := h.kubeManager.FetchUserData(context.TODO())
	if err != nil {
		c.Logger().Error(err)
		return err
	}

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

	secretData := sessionIDSecret.Data[authCode.SessionID]

	if secretData == nil {
		http.Error(c.Response(), "unexpected state", http.StatusBadRequest)
		c.Logger().Error(err)
		return nil
	}

	_, err = h.oidc.OIDCProviderConfig(nil).Exchange(context.TODO(), code)
	if err != nil {
		http.Error(c.Response(), fmt.Sprintf("failed to get token: %v", err), http.StatusInternalServerError)
		c.Logger().Error(err)
		return err
	}

	kfg, err := h.kubeManager.HandleResources(context.TODO())
	if err != nil {
		c.Logger().Error(err)
	}

	authResponse := resources.AuthResponse{
		SessionID:   authCode.SessionID,
		Kubeconfig:  kfg,
		ClusterName: h.clusterName,
	}

	payload, err := json.Marshal(authResponse)
	if err != nil {
		c.Logger().Error(err)
		return err
	}

	encoded := base64.StdEncoding.EncodeToString(payload)

	parsedAuthURL, err := url.Parse(authCode.RedirectURL)
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
