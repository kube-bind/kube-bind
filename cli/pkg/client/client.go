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

package client

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/kube-bind/kube-bind/cli/pkg/config"
	kubebindv1alpha2 "github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2"
)

type Client interface {
	GetTemplates(ctx context.Context) (*kubebindv1alpha2.APIServiceExportTemplateList, error)
	GetCollections(ctx context.Context) (*kubebindv1alpha2.CollectionList, error)
	Bind(ctx context.Context, request *kubebindv1alpha2.BindableResourcesRequest) (*kubebindv1alpha2.BindingResourceResponse, error)

	Get(urlStr string) (*http.Response, error)
	Post(urlStr string, body io.Reader) (*http.Response, error)
	Do(req *http.Request) (*http.Response, error)
}

// authenticatedClient provides an HTTP client with automatic authentication
type authenticatedClient struct {
	client *http.Client
	server config.Server

	insecure bool
}

type ClientOption func(*authenticatedClient)

// WithInsecure configures the client to skip TLS certificate verification
// WARNING: This should only be used for testing or development environments
func WithInsecure(insecure bool) ClientOption {
	return func(c *authenticatedClient) {
		c.insecure = insecure
	}
}

// NewClient creates a new authenticated HTTP client
func NewClient(server config.Server, opts ...ClientOption) (Client, error) {
	authClient := &authenticatedClient{
		server: server,
	}
	for _, opt := range opts {
		opt(authClient)
	}

	if authClient.client == nil {
		authClient.client = &http.Client{}
	}

	// Create an insecure HTTP client if needed
	if authClient.insecure {
		authClient.client.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		}
	}

	return authClient, nil
}

func (c *authenticatedClient) GetTemplates(ctx context.Context) (*kubebindv1alpha2.APIServiceExportTemplateList, error) {
	url, err := c.buildEndpointURL("templates")
	if err != nil {
		return nil, fmt.Errorf("failed to build templates URL: %w", err)
	}

	resp, err := c.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result kubebindv1alpha2.APIServiceExportTemplateList
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode templates response: %w", err)
	}

	return &result, nil
}

func (c *authenticatedClient) GetCollections(ctx context.Context) (*kubebindv1alpha2.CollectionList, error) {
	url, err := c.buildEndpointURL("collections")
	if err != nil {
		return nil, fmt.Errorf("failed to build collections URL: %w", err)
	}

	resp, err := c.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result kubebindv1alpha2.CollectionList
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode collections response: %w", err)
	}

	return &result, nil
}

func (c *authenticatedClient) Bind(ctx context.Context, request *kubebindv1alpha2.BindableResourcesRequest) (*kubebindv1alpha2.BindingResourceResponse, error) {
	url, err := c.buildEndpointURL("bind")
	if err != nil {
		return nil, fmt.Errorf("failed to build bind URL: %w", err)
	}

	reqBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal bind request: %w", err)
	}

	resp, err := c.Post(url, io.NopCloser(bytes.NewReader(reqBody)))
	if err != nil {
		return nil, fmt.Errorf("failed to send bind request: %w", err)
	}
	defer resp.Body.Close()

	var result kubebindv1alpha2.BindingResourceResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode bind response: %w", err)
	}

	return &result, nil
}

// Get performs an authenticated GET request
func (c *authenticatedClient) Get(urlStr string) (*http.Response, error) {
	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		return nil, err
	}

	if err := c.addAuthHeaders(req); err != nil {
		return nil, err
	}

	return c.client.Do(req)
}

// Post performs an authenticated POST request
func (c *authenticatedClient) Post(urlStr string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest("POST", urlStr, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	if err := c.addAuthHeaders(req); err != nil {
		return nil, err
	}

	return c.client.Do(req)
}

// Do performs an authenticated HTTP request
func (c *authenticatedClient) Do(req *http.Request) (*http.Response, error) {
	if err := c.addAuthHeaders(req); err != nil {
		return nil, err
	}

	return c.client.Do(req)
}

// addAuthHeaders adds authentication headers to the request
func (c *authenticatedClient) addAuthHeaders(req *http.Request) error {
	s := c.server
	// Check if we have stored authentication for this server
	if !c.isTokenValid() {
		err := fmt.Errorf("no valid authentication found for server %s. Please run 'kubectl bind login %s' first", s.URL, s.URL)
		if s.Cluster != "" {
			err = fmt.Errorf("no valid authentication found for server %s with cluster %s. Please run 'kubectl bind login %s --cluster %s' first", s.URL, s.Cluster, s.URL, s.Cluster)
		}
		return err
	}

	req.Header.Set("Authorization", c.getAuthorizationHeader())
	req.Header.Set("User-Agent", "kubectl-bind-cli")
	req.Header.Set("Accept", "application/json")

	return nil
}

// isTokenValid checks if the token for the given server is still valid
func (c *authenticatedClient) isTokenValid() bool {

	if c.server.AccessToken == "" {
		return false
	}

	// Check if token has expired (with 5 minute buffer)
	if !c.server.ExpiresAt.IsZero() && time.Now().Add(5*time.Minute).After(c.server.ExpiresAt) {
		return false
	}

	return true
}

// buildEndpointURL constructs an endpoint URL based on server config and cluster context
func (c *authenticatedClient) buildEndpointURL(endpoint string) (string, error) {
	baseURL, err := url.Parse(c.server.URL)
	if err != nil {
		return "", fmt.Errorf("invalid server URL %q: %w", c.server.URL, err)
	}
	if baseURL.Scheme == "" || baseURL.Host == "" {
		return "", fmt.Errorf("invalid server URL %q: missing scheme or host", c.server.URL)
	}

	baseURL.Path = path.Join(baseURL.Path, "api", strings.TrimPrefix(endpoint, "/"))

	if c.server.Cluster != "" {
		query := baseURL.Query()
		query.Set("cluster_id", c.server.Cluster)
		baseURL.RawQuery = query.Encode()
	}

	return baseURL.String(), nil
}

// getAuthorizationHeader returns the authorization header value for the given server
func (c *authenticatedClient) getAuthorizationHeader() string {
	tokenType := c.server.TokenType
	if tokenType == "" {
		tokenType = "Bearer"
	}

	return fmt.Sprintf("%s %s", tokenType, c.server.AccessToken)
}
