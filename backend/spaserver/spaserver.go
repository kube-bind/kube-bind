/*
Copyright 2025 The Kube Bind Authors.

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

package spaserver

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

type SPAFileServer struct {
	fileSystem http.FileSystem
	fileServer http.Handler
}

func NewSPAFileServer(fileSystem http.FileSystem) *SPAFileServer {
	return &SPAFileServer{
		fileSystem: fileSystem,
		fileServer: http.FileServer(fileSystem),
	}
}

func (s *SPAFileServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path, err := filepath.Abs(r.URL.Path)
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	// Try to serve the file directly first
	if f, err := s.fileSystem.Open(path); err == nil {
		if err = f.Close(); err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		s.fileServer.ServeHTTP(w, r)
		return
	} else if !os.IsNotExist(err) {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// File not found - check if it's a static asset
	if isStaticAsset(path) {
		if f, err := s.fileSystem.Open(path); err == nil {
			if err = f.Close(); err != nil {
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}
			r.URL.Path = path
			s.fileServer.ServeHTTP(w, r)
			return
		}
		// Asset not found even with clean path, return 404
		http.NotFound(w, r)
		return
	}

	// For navigation routes, serve index.html
	r.URL.Path = "/"
	s.fileServer.ServeHTTP(w, r)
}

// isStaticAsset checks if the path is for a static asset.
func isStaticAsset(path string) bool {
	return strings.Contains(path, "/assets/") ||
		strings.HasSuffix(path, ".js") ||
		strings.HasSuffix(path, ".css") ||
		strings.HasSuffix(path, ".png") ||
		strings.HasSuffix(path, ".jpg") ||
		strings.HasSuffix(path, ".jpeg") ||
		strings.HasSuffix(path, ".gif") ||
		strings.HasSuffix(path, ".svg") ||
		strings.HasSuffix(path, ".ico") ||
		strings.HasSuffix(path, ".woff") ||
		strings.HasSuffix(path, ".woff2") ||
		strings.HasSuffix(path, ".ttf") ||
		strings.HasSuffix(path, ".eot")
}

// SPAReverseProxyServer is used for local development or in theory it could be used.
type SPAReverseProxyServer struct {
	reverseProxy *httputil.ReverseProxy
}

func NewSPAReverseProxyServer(frontend string) (*SPAReverseProxyServer, error) {
	u, err := url.Parse(frontend)
	if err != nil {
		return nil, err
	}

	return &SPAReverseProxyServer{
		reverseProxy: httputil.NewSingleHostReverseProxy(u),
	}, nil
}

func (s *SPAReverseProxyServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.reverseProxy.ServeHTTP(w, r)
}
