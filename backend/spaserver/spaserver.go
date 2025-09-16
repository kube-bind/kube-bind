package spaserver

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
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

	if f, err := s.fileSystem.Open(path); err == nil {
		if err = f.Close(); err != nil {
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		s.fileServer.ServeHTTP(w, r)
	} else if os.IsNotExist(err) {
		r.URL.Path = ""
		s.fileServer.ServeHTTP(w, r)
		return
	} else {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// SPAReverseProxyServer is used for local development or in theory it could be used
// if the frontend is running on a different container
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
