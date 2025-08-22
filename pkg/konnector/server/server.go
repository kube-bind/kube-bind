/*
Copyright 2023 The Kube Bind Authors.

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

package server

import (
	"context"
	"net"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"k8s.io/klog/v2"
)

const listenAddr = ":8090"

type Server struct {
	Router  *mux.Router
	Checker checker

	ServerAddr string
}

type Config struct {
	ServerAddr string
}

func NewServer(cfg Config) *Server {
	server := &Server{
		Router:     mux.NewRouter(),
		Checker:    checker{},
		ServerAddr: cfg.ServerAddr,
	}
	server.Router.Handle("/healthz", server.Checker)

	if server.ServerAddr == "" {
		server.ServerAddr = listenAddr
	}

	return server
}

func (s *Server) Start(ctx context.Context) {
	s.Router.Handle("/healthz", s.Checker)
	server := &http.Server{
		Handler:           s.Router,
		ReadHeaderTimeout: 1 * time.Minute,
	}
	go func() {
		//nolint:gosec
		listener, err := net.Listen("tcp", s.ServerAddr)
		if err != nil {
			panic(err)
		}

		log := klog.FromContext(ctx)
		log.Info("healthz endpoint listening", "address", listener.Addr())

		server.Serve(listener) //nolint:errcheck
	}()
	go func() {
		<-ctx.Done()
		server.Close()
	}()
}
