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
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestCreateNamespace(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	tests := []struct {
		name              string
		generateName      string
		id                string
		wantErr           bool
		wantNameGenerated bool
		wantAnnotations   map[string]string
	}{
		{
			name:              "create new namespace with generateName",
			generateName:      "test-ns",
			id:                "test-id-123",
			wantErr:           false,
			wantNameGenerated: true,
			wantAnnotations: map[string]string{
				IdentityAnnotationKey: "test-id-123",
			},
		},
		{
			name:              "create new namespace with generateName already ending with dash",
			generateName:      "test-ns-",
			id:                "test-id-456",
			wantErr:           false,
			wantNameGenerated: true,
			wantAnnotations: map[string]string{
				IdentityAnnotationKey: "test-id-456",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			cl := fake.NewClientBuilder().WithScheme(scheme).Build()

			result, err := CreateNamespace(ctx, cl, tt.generateName, tt.id)

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

			expectedGenerateNamePrefix := tt.generateName
			if !strings.HasSuffix(expectedGenerateNamePrefix, "-") {
				expectedGenerateNamePrefix += "-"
			}

			if tt.wantNameGenerated {
				if !strings.HasPrefix(result.Name, expectedGenerateNamePrefix) {
					t.Errorf("CreateNamespace() name = %v, expected to start with %v", result.Name, expectedGenerateNamePrefix)
				}
				if result.GenerateName != expectedGenerateNamePrefix {
					t.Errorf("CreateNamespace() generateName = %v, expected %v", result.GenerateName, expectedGenerateNamePrefix)
				}
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

func TestCreateNamespaceLegacyAnnotationHandling(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	tests := []struct {
		name                string
		existingNamespace   *corev1.Namespace
		generateName        string
		id                  string
		wantErr             bool
		wantErrType         func(error) bool
		wantLegacyMigration bool
		wantAnnotations     map[string]string
	}{
		{
			name: "namespace exists with matching current annotation - no migration needed",
			existingNamespace: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-ns-abc123",
					Annotations: map[string]string{
						IdentityAnnotationKey: "matching-id",
					},
				},
			},
			generateName: "test-ns",
			id:           "matching-id",
			wantErr:      false,
			wantAnnotations: map[string]string{
				IdentityAnnotationKey: "matching-id",
			},
			wantLegacyMigration: false,
		},
		{
			name: "namespace exists with matching legacy annotation - should migrate",
			existingNamespace: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-ns-xyz789",
					Annotations: map[string]string{
						legacyIdentityAnnotationKey: "legacy-id",
					},
				},
			},
			generateName: "test-ns",
			id:           "legacy-id",
			wantErr:      false,
			wantAnnotations: map[string]string{
				IdentityAnnotationKey: "legacy-id",
			},
			wantLegacyMigration: true,
		},
		{
			name: "namespace exists with both annotations, legacy matches - should migrate",
			existingNamespace: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-ns-mixed",
					Annotations: map[string]string{
						IdentityAnnotationKey:       "wrong-id",
						legacyIdentityAnnotationKey: "correct-id",
					},
				},
			},
			generateName: "test-ns",
			id:           "correct-id",
			wantErr:      false,
			wantAnnotations: map[string]string{
				IdentityAnnotationKey: "correct-id",
			},
			wantLegacyMigration: true,
		},
		{
			name: "namespace exists with non-matching annotations - should return AlreadyExists error",
			existingNamespace: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-ns-conflict",
					Annotations: map[string]string{
						IdentityAnnotationKey:       "different-id",
						legacyIdentityAnnotationKey: "another-id",
					},
				},
			},
			generateName: "test-ns",
			id:           "new-id",
			wantErr:      true,
			wantErrType:  errors.IsAlreadyExists,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			interceptorClient := &alreadyExistsClient{
				Client:            fake.NewClientBuilder().WithScheme(scheme).WithObjects(tt.existingNamespace).Build(),
				existingNamespace: tt.existingNamespace,
			}

			result, err := CreateNamespace(ctx, interceptorClient, tt.generateName, tt.id)

			if tt.wantErr {
				if err == nil {
					t.Errorf("CreateNamespace() expected error but got none")
					return
				}
				if tt.wantErrType != nil && !tt.wantErrType(err) {
					t.Errorf("CreateNamespace() error type mismatch, got %T: %v", err, err)
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

			if result.Name != tt.existingNamespace.Name {
				t.Errorf("CreateNamespace() name = %v, expected %v", result.Name, tt.existingNamespace.Name)
			}

			for key, expectedValue := range tt.wantAnnotations {
				if actualValue, exists := result.Annotations[key]; !exists || actualValue != expectedValue {
					t.Errorf("CreateNamespace() annotation %s = %v, expected %v (exists: %v)", key, actualValue, expectedValue, exists)
				}
			}

			if tt.wantLegacyMigration {
				if _, exists := result.Annotations[legacyIdentityAnnotationKey]; exists {
					t.Errorf("CreateNamespace() legacy annotation should be removed but still exists: %v", result.Annotations)
				}
			}

			var actualNamespace corev1.Namespace
			if err := interceptorClient.Get(ctx, client.ObjectKey{Name: result.Name}, &actualNamespace); err != nil {
				t.Fatalf("Failed to get updated namespace from client: %v", err)
			}

			for key, expectedValue := range tt.wantAnnotations {
				if actualValue, exists := actualNamespace.Annotations[key]; !exists || actualValue != expectedValue {
					t.Errorf("CreateNamespace() stored annotation %s = %v, expected %v (exists: %v)", key, actualValue, expectedValue, exists)
				}
			}

			if tt.wantLegacyMigration {
				if _, exists := actualNamespace.Annotations[legacyIdentityAnnotationKey]; exists {
					t.Errorf("CreateNamespace() legacy annotation should be removed from stored namespace but still exists: %v", actualNamespace.Annotations)
				}
			}
		})
	}
}

type alreadyExistsClient struct {
	client.Client
	existingNamespace *corev1.Namespace
}

func (c *alreadyExistsClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	namespace, ok := obj.(*corev1.Namespace)
	if !ok {
		return c.Client.Create(ctx, obj, opts...)
	}

	if namespace.GenerateName != "" {
		namespace.Name = c.existingNamespace.Name
		return errors.NewAlreadyExists(corev1.Resource("namespace"), namespace.Name)
	}

	return c.Client.Create(ctx, obj, opts...)
}
