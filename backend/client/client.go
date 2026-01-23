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
	"net/http"
	"net/url"
)

type ClientParameters struct {
	ClusterID   string
	RedirectURL string
	SessionID   string
	ClientType  string
	ConsumerID  string

	IsClusterScoped bool
}

// GetQueryParams extracts the client parameters from the given HTTP request.
func GetQueryParams(r *http.Request) *ClientParameters {
	p := &ClientParameters{
		ClusterID:   r.URL.Query().Get("cluster_id"),
		RedirectURL: r.URL.Query().Get("redirect_url"),
		SessionID:   r.URL.Query().Get("session_id"),
		ClientType:  r.URL.Query().Get("client_type"),
		ConsumerID:  r.URL.Query().Get("consumer_id"),
	}
	p.IsClusterScoped = p.ClusterID != ""
	return p
}

// WithParams adds the client parameters to the given URL as query parameters.
func (r *ClientParameters) WithParams(urlStr string) string {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		// Return original URL if parsing fails
		return urlStr
	}

	query := parsedURL.Query()

	// Add all non-empty client parameters
	if r.ClusterID != "" {
		query.Set("cluster_id", r.ClusterID)
	}
	if r.SessionID != "" {
		query.Set("session_id", r.SessionID)
	}
	if r.ClientType != "" {
		query.Set("client_type", r.ClientType)
	}
	if r.ConsumerID != "" {
		query.Set("consumer_id", r.ConsumerID)
	}

	parsedURL.RawQuery = query.Encode()
	return parsedURL.String()
}
