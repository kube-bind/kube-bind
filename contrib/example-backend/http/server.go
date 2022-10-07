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

package http

import (
	"fmt"
	"github.com/labstack/echo/v4"
)

type server struct {
	address string
	port    int

	handler *handler

	router *echo.Echo
}

func NewServer(address string, port int, handler *handler) (*server, error) {
	if address == "" || port == 0 {
		return nil, fmt.Errorf("server address or port cannot be empty")
	}
	router := echo.New()

	server := &server{
		address: address,
		port:    port,
		handler: handler,
		router:  router,
	}

	server.router.GET("/authorize", server.handler.handleAuthorize)
	server.router.GET("/callback", server.handler.handleCallback)

	return server, nil
}

func (s *server) Start() {
	s.router.Logger.Fatal(s.router.Start(fmt.Sprintf("%v:%v", s.address, s.port)))
}
