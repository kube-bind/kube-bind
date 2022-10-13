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

package authenticator

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"time"

	"github.com/labstack/echo/v4"

	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/kube-bind/kube-bind/contrib/example-backend/kubernetes/resources"
)

type Authenticator interface {
	Endpoint(context.Context) string
	Execute(context.Context) error
}

type defaultAuthenticator struct {
	server  *echo.Echo
	port    int
	timeout time.Duration
	action  func(string, string, string) error
}

func NewDefaultAuthenticator(timeout time.Duration, action func(clusterName, sessionID, kubeconfig string) error) (Authenticator, error) {
	if timeout == 0 {
		timeout = 2 * time.Second
	}

	defaultAuthenticator := &defaultAuthenticator{
		timeout: timeout,
		action:  action,
	}

	server := echo.New()
	server.HideBanner = true
	server.GET("/", defaultAuthenticator.actionWrapper())
	defaultAuthenticator.server = server

	address, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		return nil, err
	}

	listener, err := net.ListenTCP("tcp", address)
	if err != nil {
		return nil, err
	}

	if err = listener.Close(); err != nil {
		return nil, err
	}

	defaultAuthenticator.port = listener.Addr().(*net.TCPAddr).Port

	return defaultAuthenticator, nil
}

func (d *defaultAuthenticator) Execute(ctx context.Context) error {
	return wait.PollImmediateWithContext(ctx, d.timeout, d.timeout,
		func(ctx context.Context) (done bool, err error) {
			return false, d.server.Start(fmt.Sprintf("localhost:%v", d.port))
		})
}

func (d *defaultAuthenticator) Endpoint(context.Context) string {
	return fmt.Sprintf("http://localhost:%v", d.port)
}

func (d *defaultAuthenticator) actionWrapper() func(echo.Context) error {
	return func(c echo.Context) error {
		authData := c.QueryParam("auth_response")

		decode, err := base64.StdEncoding.DecodeString(authData)
		if err != nil {
			c.Logger().Error(err)
			return err
		}

		authResponse := &resources.AuthResponse{}
		if err := json.Unmarshal(decode, authResponse); err != nil {
			return err
		}

		if err := d.action(authResponse.ClusterName, authResponse.SessionID, string(authResponse.Kubeconfig)); err != nil {
			return err
		}

		if _, err := fmt.Fprintf(c.Response(), "<h1>Successfully Authentication! Please head back to the command line</h1>"); err != nil {
			return err
		}

		time.Sleep(5 * time.Second)
		d.server.Server.Close()
		return nil
	}
}
