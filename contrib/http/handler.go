/*
Copyright 2022 The Kubectl Bind API contributors.

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
	"errors"
	"fmt"
	"github.com/kube-bind/kube-bind/contrib/kubernetes"
	"net/http"

	echo2 "github.com/labstack/echo/v4"
)

type handler struct {
	oidc *oidcServiceProvider

	client *http.Client

	kubeManager *kubernetes.Manager
}

func NewHandler(provider *oidcServiceProvider, mgr *kubernetes.Manager) (*handler, error) {
	return &handler{
		oidc:        provider,
		client:      http.DefaultClient,
		kubeManager: mgr,
	}, nil
}

func (h *handler) handleAuthorize(c echo2.Context) error {
	var scopes = []string{"openid", "profile", "email", "offline_access"}
	authURL := h.oidc.OIDCProviderConfig(scopes).AuthCodeURL("hello-kube-bind")
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
	if state := c.FormValue("state"); state != "hello-kube-bind" {
		http.Error(c.Response(), fmt.Sprintf("expected state %q got %q", "hello-kube-bind", state), http.StatusBadRequest)
		return nil
	}
	_, err := h.oidc.OIDCProviderConfig(nil).Exchange(context.TODO(), code)
	if err != nil {
		http.Error(c.Response(), fmt.Sprintf("failed to get token: %v", err), http.StatusInternalServerError)
		return err
	}

	kfg, err := h.kubeManager.HandleResources(context.TODO())
	if err != nil {
		c.Logger().Error(err)
	}

	_, err = c.Response().Writer.Write(kfg)
	return err
}
