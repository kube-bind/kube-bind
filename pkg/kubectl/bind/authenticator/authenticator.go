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
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	kubebindv1alpha1 "github.com/kube-bind/kube-bind/pkg/apis/kubebind/v1alpha1"
)

var (
	kubebindSchema = runtime.NewScheme()
	kubebindCodecs = serializer.NewCodecFactory(kubebindSchema)
)

func init() {
	utilruntime.Must(kubebindv1alpha1.AddToScheme(kubebindSchema))
}

type Authenticator interface {
	Endpoint(context.Context) string
	Execute(context.Context) error
}

type defaultAuthenticator struct {
	server  *echo.Echo
	port    int
	timeout time.Duration
	action  func(context.Context, schema.GroupVersionKind, runtime.Object) error
}

func NewDefaultAuthenticator(timeout time.Duration, action func(context.Context, schema.GroupVersionKind, runtime.Object) error) (Authenticator, error) {
	if timeout == 0 {
		timeout = 5 * time.Second
	}

	defaultAuthenticator := &defaultAuthenticator{
		timeout: timeout,
		action:  action,
	}

	server := echo.New()
	server.HideBanner = true
	server.GET("/callback", defaultAuthenticator.actionWrapper())
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
	go func() {
		time.Sleep(10 * time.Minute)
		d.server.Server.Close()
	}()

	return d.server.Start(net.JoinHostPort("localhost", strconv.Itoa(d.port)))
}

func (d *defaultAuthenticator) Endpoint(context.Context) string {
	return fmt.Sprintf("http://%s/callback", net.JoinHostPort("localhost", strconv.Itoa(d.port)))
}

func (d *defaultAuthenticator) actionWrapper() func(echo.Context) error {
	return func(c echo.Context) error {
		authData := c.QueryParam("response")

		fmt.Printf("Got callback\n")

		decoded, err := base64.StdEncoding.DecodeString(authData)
		if err != nil {
			c.Logger().Error(err)
			return err
		}

		authResponse, gvk, err := kubebindCodecs.UniversalDeserializer().Decode(decoded, nil, nil)
		if err != nil {
			c.Logger().Error(err)
			return err
		}
		if err := d.action(c.Request().Context(), *gvk, authResponse); err != nil {
			return err
		}

		if _, err := c.Response().Write([]byte("<h1>Successfully Authentication! Please head back to the command line</h1>")); err != nil {
			return err
		}

		go func() {
			time.Sleep(5 * time.Second)
			d.server.Server.Close() // nolint:errcheck
		}()

		return nil
	}
}
