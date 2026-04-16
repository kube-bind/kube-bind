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

package claimedresources

import (
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	konnectortypes "github.com/kube-bind/kube-bind/pkg/konnector/types"
)

func TestSetSourceMetadataAnnotations(t *testing.T) {
	tests := []struct {
		name                string
		obj                 *unstructured.Unstructured
		sourceNS            string
		sourceUID           string
		nsKey               string
		uidKey              string
		expectedAnnotations map[string]string
	}{
		{
			name:      "provider annotations on empty object",
			obj:       &unstructured.Unstructured{},
			sourceNS:  "provider-ns",
			sourceUID: "provider-uid-123",
			nsKey:     konnectortypes.ProviderNamespaceAnnotationKey,
			uidKey:    konnectortypes.ProviderUIDAnnotationKey,
			expectedAnnotations: map[string]string{
				konnectortypes.ProviderNamespaceAnnotationKey: "provider-ns",
				konnectortypes.ProviderUIDAnnotationKey:       "provider-uid-123",
			},
		},
		{
			name:      "consumer annotations on empty object",
			obj:       &unstructured.Unstructured{},
			sourceNS:  "consumer-ns",
			sourceUID: "consumer-uid-456",
			nsKey:     konnectortypes.ConsumerNamespaceAnnotationKey,
			uidKey:    konnectortypes.ConsumerUIDAnnotationKey,
			expectedAnnotations: map[string]string{
				konnectortypes.ConsumerNamespaceAnnotationKey: "consumer-ns",
				konnectortypes.ConsumerUIDAnnotationKey:       "consumer-uid-456",
			},
		},
		{
			name: "preserves existing annotations",
			obj: func() *unstructured.Unstructured {
				obj := &unstructured.Unstructured{}
				obj.SetAnnotations(map[string]string{
					"existing-key": "existing-value",
				})
				return obj
			}(),
			sourceNS:  "my-ns",
			sourceUID: "my-uid",
			nsKey:     konnectortypes.ProviderNamespaceAnnotationKey,
			uidKey:    konnectortypes.ProviderUIDAnnotationKey,
			expectedAnnotations: map[string]string{
				konnectortypes.ProviderNamespaceAnnotationKey: "my-ns",
				konnectortypes.ProviderUIDAnnotationKey:       "my-uid",
				"existing-key":                                "existing-value",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			konnectortypes.SetSourceMetadataAnnotations(tt.obj, tt.sourceNS, tt.sourceUID, tt.nsKey, tt.uidKey)

			annotations := tt.obj.GetAnnotations()
			for key, expected := range tt.expectedAnnotations {
				require.Equal(t, expected, annotations[key], "annotation %s mismatch", key)
			}
		})
	}
}

func TestCandidateFromOwnerObjPreservesKubeBindAnnotations(t *testing.T) {
	obj := &unstructured.Unstructured{}
	obj.SetAnnotations(map[string]string{
		konnectortypes.ProviderNamespaceAnnotationKey: "provider-ns",
		konnectortypes.ProviderUIDAnnotationKey:       "provider-uid",
		konnectortypes.ConsumerNamespaceAnnotationKey: "consumer-ns",
		konnectortypes.ConsumerUIDAnnotationKey:       "consumer-uid",
		"kcp.io/cluster":                              "should-be-stripped",
		"user-annotation":                             "should-be-kept",
	})
	obj.SetNamespace("original-ns")
	obj.SetName("test-obj")

	candidate := candidateFromOwnerObj("target-ns", obj)

	annotations := candidate.GetAnnotations()
	require.Equal(t, "provider-ns", annotations[konnectortypes.ProviderNamespaceAnnotationKey])
	require.Equal(t, "provider-uid", annotations[konnectortypes.ProviderUIDAnnotationKey])
	require.Equal(t, "consumer-ns", annotations[konnectortypes.ConsumerNamespaceAnnotationKey])
	require.Equal(t, "consumer-uid", annotations[konnectortypes.ConsumerUIDAnnotationKey])
	require.Equal(t, "should-be-kept", annotations["user-annotation"])
	_, hasKcp := annotations["kcp.io/cluster"]
	require.False(t, hasKcp, "kcp.io/cluster annotation should be stripped")
}
