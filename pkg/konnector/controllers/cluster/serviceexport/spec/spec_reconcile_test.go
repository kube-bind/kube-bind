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

package spec

import (
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	konnectortypes "github.com/kube-bind/kube-bind/pkg/konnector/types"
)

func TestInjectClusterNamespace(t *testing.T) {
	tests := []struct {
		name         string
		obj          *unstructured.Unstructured
		clusterNs    string
		clusterNsUID string
		expected     string
		wantErr      bool
	}{
		{
			name:         "noExistingClusterNs",
			obj:          &unstructured.Unstructured{},
			clusterNs:    "kube-bind-zlp9m",
			clusterNsUID: "real-identity",
			expected:     "kube-bind-zlp9m",
			wantErr:      false,
		},
		{
			name:         "oneExistingClusterNs",
			obj:          newObjectWithClusterNs("kube-bind-zlp9m"),
			clusterNs:    "kube-bind-s85lc",
			clusterNsUID: "real-identity",
			expected:     "kube-bind-zlp9m",
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			originalClusterAnn := tt.obj.GetAnnotations()[konnectortypes.ClusterNamespaceAnnotationKey]

			rec := &reconciler{
				clusterNamespace: tt.clusterNs,
			}

			err := rec.setClusterNamespaceAnnotation(tt.obj)
			if tt.wantErr {
				require.Error(t, err)

				// ensure object was not modified
				require.Equal(t, originalClusterAnn, tt.obj.GetAnnotations()[konnectortypes.ClusterNamespaceAnnotationKey])
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.clusterNs, tt.obj.GetAnnotations()[konnectortypes.ClusterNamespaceAnnotationKey])
			}
		})
	}
}

func TestSetSourceAnnotations(t *testing.T) {
	tests := []struct {
		name              string
		obj               *unstructured.Unstructured
		consumerNamespace string
		consumerUID       string
	}{
		{
			name:              "no existing annotations",
			obj:               &unstructured.Unstructured{},
			consumerNamespace: "my-namespace",
			consumerUID:       "abc-123-def",
		},
		{
			name:              "with existing cluster namespace annotation",
			obj:               newObjectWithClusterNs("kube-bind-zlp9m"),
			consumerNamespace: "other-namespace",
			consumerUID:       "xyz-456-ghi",
		},
		{
			name:              "cluster-scoped object with empty namespace",
			obj:               &unstructured.Unstructured{},
			consumerNamespace: "",
			consumerUID:       "uid-789",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rec := &reconciler{}
			rec.setSourceAnnotations(tt.obj, tt.consumerNamespace, tt.consumerUID)

			annotations := tt.obj.GetAnnotations()
			require.Equal(t, tt.consumerNamespace, annotations[konnectortypes.ConsumerNamespaceAnnotationKey])
			require.Equal(t, tt.consumerUID, annotations[konnectortypes.ConsumerUIDAnnotationKey])
		})
	}
}

func newObjectWithClusterNs(providerNamespace string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	ans := map[string]string{
		konnectortypes.ClusterNamespaceAnnotationKey: providerNamespace,
	}
	obj.SetAnnotations(ans)
	ors := []metav1.OwnerReference{{
		APIVersion: "v1",
		Kind:       "Namespace",
		Name:       providerNamespace,
	}}
	obj.SetOwnerReferences(ors)

	return obj
}
