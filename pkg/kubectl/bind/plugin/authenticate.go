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

package plugin

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"

	kubebindv1alpha1 "github.com/kube-bind/kube-bind/pkg/apis/kubebind/v1alpha1"
	"github.com/kube-bind/kube-bind/pkg/kubectl/bind/util"
)

func fetchAuthenticationRoute(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}

	blob, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if err := resp.Body.Close(); err != nil {
		return "", err
	}

	data := &kubebindv1alpha1.APIServiceProvider{}
	if err := json.Unmarshal(blob, data); err != nil {
		return "", err
	}

	return data.Spec.AuthenticatedClientURL, nil
}

func authenticate(parsedAuthURL *url.URL, authEndpoint, sessionID string) error {
	values := parsedAuthURL.Query()
	values.Add("redirect_url", authEndpoint)
	values.Add("session_id", sessionID)

	parsedAuthURL.RawQuery = values.Encode()

	return util.OpenBrowser(parsedAuthURL.String())
}
