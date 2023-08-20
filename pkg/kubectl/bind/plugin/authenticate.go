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
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/blang/semver/v4"
	"github.com/mdp/qrterminal/v3"

	clientgoversion "k8s.io/client-go/pkg/version"

	kubebindv1alpha1 "github.com/kube-bind/kube-bind/pkg/apis/kubebind/v1alpha1"
	"github.com/kube-bind/kube-bind/pkg/version"
)

func getProvider(url string) (*kubebindv1alpha1.BindingProvider, error) {
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

	provider := &kubebindv1alpha1.BindingProvider{}
	if err := json.Unmarshal(blob, provider); err != nil {
		return nil, err
	}

	// check provider version compatibility
	bindVersion, err := version.BinaryVersion(clientgoversion.Get().GitVersion)
	if err != nil {
		return nil, err
	}
	if bindSemVer, err := semver.Parse(strings.TrimLeft(bindVersion, "v")); err != nil {
		return nil, fmt.Errorf("failed to parse bind version %q: %v", bindVersion, err)
	} else if min := semver.MustParse("0.3.0"); bindSemVer.GE(min) {
		// we added this in v0.3.0. Don't test before.
		if err := validateProviderVersion(provider.Version); err != nil {
			return nil, err
		}
	}

	return provider, nil
}

func validateProviderVersion(providerVersion string) error {
	if providerVersion == "" {
		return fmt.Errorf("provider version %q is empty, please update the backend to v0.3.0+", providerVersion)
	} else if providerVersion == "v0.0.0" || providerVersion == "v0.0.0-master+$Format:%H$" {
		// unversioned, development version
		return nil
	}

	providerSemVer, err := semver.Parse(strings.TrimPrefix(providerVersion, "v"))
	if err != nil {
		return fmt.Errorf("provider version %q cannot be parsed", providerVersion)
	}
	if min := semver.MustParse("0.3.0"); providerSemVer.LT(min) {
		return fmt.Errorf("provider version %s is not supported, must be at least v%s", providerVersion, min)
	}

	return nil
}

func (b *BindOptions) authenticate(provider *kubebindv1alpha1.BindingProvider, callback, sessionID, clusterID string, urlCh chan<- string) error {
	var oauth2Method *kubebindv1alpha1.OAuth2CodeGrant
	for _, m := range provider.AuthenticationMethods {
		if m.Method == "OAuth2CodeGrant" {
			oauth2Method = m.OAuth2CodeGrant
			break
		}
	}
	if oauth2Method == nil {
		return errors.New("server does not support OAuth2 code grant flow")
	}

	u, err := url.Parse(oauth2Method.AuthenticatedURL)
	if err != nil {
		return fmt.Errorf("failed to parse auth url: %v", err)
	}

	cbURL, err := url.Parse(callback)
	if err != nil {
		return fmt.Errorf("failed to parse callback url: %v", err)
	}
	_, cbPort, err := net.SplitHostPort(cbURL.Host)
	if err != nil {
		return fmt.Errorf("failed to parse callback port: %v", err)
	}

	values := u.Query()
	values.Add("p", cbPort)
	values.Add("s", sessionID)
	values.Add("c", clusterID)
	u.RawQuery = values.Encode()

	fmt.Fprintf(b.Options.ErrOut, "\nTo authenticate, visit in your browser:\n\n\t%s\n", u.String()) // nolint: errcheck

	// TODO(sttts): callback backend, not 127.0.0.1
	if false {
		fmt.Fprintf(b.Options.ErrOut, "\n\nor scan the QRCode below:")
		config := qrterminal.Config{
			Level:     qrterminal.L,
			Writer:    b.Options.ErrOut,
			BlackChar: qrterminal.WHITE,
			WhiteChar: qrterminal.BLACK,
			QuietZone: 2,
		}
		qrterminal.GenerateWithConfig(u.String(), config)
	}
	if urlCh != nil {
		urlCh <- u.String()
	}

	return nil
}
