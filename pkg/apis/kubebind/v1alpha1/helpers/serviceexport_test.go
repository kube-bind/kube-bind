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

package helpers

import (
	"testing"

	"github.com/stretchr/testify/require"

	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/utils/pointer"
)

func TestWebhookCRDStorageVersion(t *testing.T) {
	input := v1.CustomResourceDefinition{
		Spec: v1.CustomResourceDefinitionSpec{
			Versions: []v1.CustomResourceDefinitionVersion{
				{
					Served:  true,
					Name:    "v1alpha1",
					Storage: false,
				},
				{
					Served:  true,
					Name:    "v1",
					Storage: true,
				},
			},
			Conversion: &v1.CustomResourceConversion{
				Strategy: v1.WebhookConverter,
				Webhook: &v1.WebhookConversion{
					ClientConfig: &v1.WebhookClientConfig{
						URL:      pointer.String("https://example.com/webhook"),
						CABundle: []byte("1234")},
				},
			},
		},
	}

	output, err := CRDToServiceExport(&input)

	require.NoError(t, err)

	atLeastOneStorageVersion := false
	for _, v := range output.Versions {
		atLeastOneStorageVersion = atLeastOneStorageVersion || v.Storage
	}
	if !atLeastOneStorageVersion {
		t.Fatal("returned ResourceExport has no storage version", output)
	}
}
