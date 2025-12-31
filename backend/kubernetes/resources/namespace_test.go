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

package resources

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"

	"github.com/martinlindhe/base36"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func expectedIdentityHash(identity string) string {
	hash := sha256.Sum224([]byte(identity))
	return strings.ToLower(base36.EncodeBytes(hash[:8]))
}

func TestCreateNamespace(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	tests := []struct {
		name            string
		generateName    string
		identity        string
		author          string
		wantErr         bool
		wantAnnotations map[string]string
	}{
		{
			name:         "create new namespace with generateName",
			generateName: "test-ns",
			identity:     "test-id-123",
			author:       "bob",
			wantErr:      false,
			wantAnnotations: map[string]string{
				IdentityAnnotationKey: "test-id-123",
				AuthorAnnotationKey:   "bob",
			},
		},
		{
			name:         "create new namespace with generateName already ending with dash",
			generateName: "test-ns",
			identity:     "test-id-456",
			author:       "alice",
			wantErr:      false,
			wantAnnotations: map[string]string{
				IdentityAnnotationKey: "test-id-456",
				AuthorAnnotationKey:   "alice",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			cl := fake.NewClientBuilder().WithScheme(scheme).Build()

			result, err := CreateNamespace(ctx, cl, tt.generateName, tt.identity, tt.author)

			if tt.wantErr {
				if err == nil {
					t.Errorf("CreateNamespace() expected error but got none")
					return
				}
				return
			}

			if err != nil {
				t.Errorf("CreateNamespace() unexpected error = %v", err)
				return
			}

			if result == nil {
				t.Errorf("CreateNamespace() returned nil namespace")
				return
			}

			expectedPrefix := tt.generateName
			expectedHash := expectedIdentityHash(tt.identity)
			expectedName := fmt.Sprintf("%s%s", expectedPrefix, expectedHash)

			if result.Name != expectedName {
				t.Errorf("CreateNamespace() name = %v, expected %v", result.Name, expectedName)
			}

			if result.GenerateName != "" {
				t.Errorf("CreateNamespace() generateName = %v, expected empty string", result.GenerateName)
			}

			for key, expectedValue := range tt.wantAnnotations {
				if actualValue, exists := result.Annotations[key]; !exists || actualValue != expectedValue {
					t.Errorf("CreateNamespace() annotation %s = %v, expected %v (exists: %v)", key, actualValue, expectedValue, exists)
				}
			}

			var actualNamespace corev1.Namespace
			if err := cl.Get(ctx, client.ObjectKey{Name: result.Name}, &actualNamespace); err != nil {
				t.Fatalf("Failed to get created/updated namespace from client: %v", err)
			}

			for key, expectedValue := range tt.wantAnnotations {
				if actualValue, exists := actualNamespace.Annotations[key]; !exists || actualValue != expectedValue {
					t.Errorf("CreateNamespace() stored annotation %s = %v, expected %v (exists: %v)", key, actualValue, expectedValue, exists)
				}
			}
		})
	}
}
