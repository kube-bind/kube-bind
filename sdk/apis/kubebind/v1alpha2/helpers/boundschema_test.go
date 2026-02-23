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

package helpers

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestUnstructuredToBoundSchema_PreservesNameAndStripsMetadata(t *testing.T) {
	u := unstructured.Unstructured{}
	u.SetName("sheriffs.wildwest.dev")
	u.SetManagedFields([]metav1.ManagedFieldsEntry{
		{
			Manager:    "some-manager",
			APIVersion: "apiextensions.k8s.io/v1",
		},
	})

	got, err := UnstructuredToBoundSchema(u)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.Name != "sheriffs.wildwest.dev" {
		t.Errorf("expected Name %q, got %q", "sheriffs.wildwest.dev", got.Name)
	}

	if len(got.ManagedFields) != 0 {
		t.Errorf("expected ManagedFields to be empty, got %v", got.ManagedFields)
	}
}
