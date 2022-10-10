/*
Copyright 2022 The Kubectl Bind contributors.

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
	"fmt"
	"net"
	"time"

	"github.com/labstack/echo/v4"

	"k8s.io/apimachinery/pkg/util/wait"
)

type Authenticator interface {
	GenerateServerPort(context.Context) (int, error)
	Execute(context.Context, int) error
}

type defaultAuthenticator struct {
	server  *echo.Echo
	timeout time.Duration
	action  func(context.Context) error
}

func NewDefaultAuthenticator(timeout time.Duration, action func(context.Context) error) Authenticator {
	if timeout == 0 {
		timeout = 2 * time.Second
	}

	defaultAuthenticator := &defaultAuthenticator{
		timeout: timeout,
		action:  action,
	}

	server := echo.New()
	server.GET("/", defaultAuthenticator.actionWrapper())
	defaultAuthenticator.server = server

	return defaultAuthenticator
}

func (d *defaultAuthenticator) GenerateServerPort(ctx context.Context) (int, error) {
	address, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		return 0, err
	}

	listener, err := net.ListenTCP("tcp", address)
	if err != nil {
		return 0, err
	}

	if err = listener.Close(); err != nil {
		return 0, err
	}

	return listener.Addr().(*net.TCPAddr).Port, nil
}

func (d *defaultAuthenticator) Execute(ctx context.Context, port int) error {
	return wait.PollImmediateWithContext(ctx, d.timeout, d.timeout,
		func(ctx context.Context) (done bool, err error) {
			return false, d.server.Start(fmt.Sprintf("localhost:%v", port))
		})
}

func (d *defaultAuthenticator) actionWrapper() func(echo.Context) error {
	return func(c echo.Context) error {
		c.Get("token")
		d.action(context.TODO())

		fmt.Fprintf(c.Response(), "<h1>Successfully Authentication! Please head back to the command line</h1>")
		time.Sleep(10 * time.Second)
		return d.server.Server.Close()
	}
}
