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
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"

	"github.com/mdp/qrterminal/v3"

	kubebindv1alpha1 "github.com/kube-bind/kube-bind/pkg/apis/kubebind/v1alpha1"
)

func fetchAuthenticationRoute(url string) (*kubebindv1alpha1.APIServiceProvider, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}

	blob, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if err := resp.Body.Close(); err != nil {
		return nil, err
	}

	provider := &kubebindv1alpha1.APIServiceProvider{}
	if err := json.Unmarshal(blob, provider); err != nil {
		return nil, err
	}

	return provider, nil
}

func authenticate(provider *kubebindv1alpha1.APIServiceProvider, authEndpoint, sessionID string, urlCh chan<- string) error {
	u, err := url.Parse(provider.Spec.AuthenticatedClientURL)
	if err != nil {
		return fmt.Errorf("failed to parse auth url: %v", err)
	}

	values := u.Query()
	values.Add("u", authEndpoint)
	values.Add("s", sessionID)
	u.RawQuery = values.Encode()

	fmt.Printf("\nTo authenticate, visit %s in your browser or scan the QRCode below:\n\n", u.String())

	// TODO(sttts): callback backend, not 127.0.0.1
	config := qrterminal.Config{
		Level:     qrterminal.L,
		Writer:    os.Stdout,
		BlackChar: qrterminal.WHITE,
		WhiteChar: qrterminal.BLACK,
		QuietZone: 2,
	}
	qrterminal.GenerateWithConfig(u.String(), config)

	if urlCh != nil {
		urlCh <- u.String()
	}

	return nil
}
