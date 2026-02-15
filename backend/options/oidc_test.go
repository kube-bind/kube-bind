/*
Copyright 2026 The Kube Bind Authors.

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

package options

import (
	"testing"

	"github.com/stretchr/testify/require"

	kubebindv1alpha2 "github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2"
)

func TestOIDCValidate(t *testing.T) {
	tests := []struct {
		name    string
		options *OIDC
		wantErr bool
		errMsg  string
	}{
		{
			name: "embedded OIDC with valid issuer URL ending in /oidc",
			options: &OIDC{
				Type:               string(kubebindv1alpha2.OIDCProviderTypeEmbedded),
				IssuerClientID:     "test-client-id",
				IssuerClientSecret: "test-client-secret",
				IssuerURL:          "http://localhost:8080/oidc",
				CallbackURL:        "http://localhost:8080/callback",
			},
			wantErr: false,
		},
		{
			name: "embedded OIDC with invalid issuer URL not ending in /oidc",
			options: &OIDC{
				Type:               string(kubebindv1alpha2.OIDCProviderTypeEmbedded),
				IssuerClientID:     "test-client-id",
				IssuerClientSecret: "test-client-secret",
				IssuerURL:          "http://localhost:8080",
				CallbackURL:        "http://localhost:8080/callback",
			},
			wantErr: true,
			errMsg:  "--oidc-issuer-url must end with '/oidc' when using embedded OIDC provider",
		},
		{
			name: "embedded OIDC with trailing slash /oidc/",
			options: &OIDC{
				Type:               string(kubebindv1alpha2.OIDCProviderTypeEmbedded),
				IssuerClientID:     "test-client-id",
				IssuerClientSecret: "test-client-secret",
				IssuerURL:          "http://localhost:8080/oidc/",
				CallbackURL:        "http://localhost:8080/callback",
			},
			wantErr: true,
			errMsg:  "--oidc-issuer-url must end with '/oidc' when using embedded OIDC provider",
		},
		{
			name: "external OIDC does not require /oidc suffix",
			options: &OIDC{
				Type:               string(kubebindv1alpha2.OIDCProviderTypeExternal),
				IssuerClientID:     "test-client-id",
				IssuerClientSecret: "test-client-secret",
				IssuerURL:          "http://localhost:8080",
				CallbackURL:        "http://localhost:8080/callback",
				AllowedGroups:      []string{"admins"},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.options.Validate()

			if !tt.wantErr {
				require.NoError(t, err)
				return
			}

			require.Error(t, err)
			if tt.errMsg != "" {
				require.EqualError(t, err, tt.errMsg)
			}
		})
	}
}
