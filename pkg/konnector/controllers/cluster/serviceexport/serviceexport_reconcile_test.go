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

package serviceexport

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kubebindv1alpha2 "github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2"
	conditionsapi "github.com/kube-bind/kube-bind/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
)

func TestEnsureCRDConditionsCopiedToBoundSchema(t *testing.T) {
	tests := []struct {
		name        string
		getCRD      func(name string) (*apiextensionsv1.CustomResourceDefinition, error)
		schema      *kubebindv1alpha2.APIResourceSchema
		boundSchema *kubebindv1alpha2.BoundAPIResourceSchema
		export      *kubebindv1alpha2.APIServiceExport
		expected    *kubebindv1alpha2.BoundAPIResourceSchema
		wantErr     bool
	}{
		{
			name: "merging",
			getCRD: newGetCRD("foo-schema", newCRD("foo-schema", []apiextensionsv1.CustomResourceDefinitionCondition{
				{Type: "Something", Status: "True", Reason: "Reason", Message: "message"},
				{Type: "Established", Status: "True", Reason: "Reason", Message: "message"},
			})),
			schema: newAPIResourceSchema("foo-schema", "default", "example.com", "foos"),
			boundSchema: newBoundAPIResourceSchema("foo-schema", []conditionsapi.Condition{
				{Type: "Ready", Status: "False", Severity: "Warning", Reason: "SomethingElseWrong", Message: "something else went wrong"},
				{Type: "Established", Status: "True", Severity: "None", Reason: "Reason", Message: "message"},
				{Type: "Structural", Status: "False", Severity: "Warning", Reason: "SomethingWrong", Message: "something went wrong"},
			}),
			export: newExportWithResources("test-export", "default", []kubebindv1alpha2.APIResourceSchemaReference{
				{Name: "foo-schema", Type: "APIResourceSchema"},
			}),
			expected: newBoundAPIResourceSchema("foo-schema", []conditionsapi.Condition{
				{Type: "Ready", Status: "False", Severity: "Warning", Reason: "SomethingWrong", Message: "something went wrong"},
				{Type: "Established", Status: "True", Severity: "", Reason: "Reason", Message: "message"},
				{Type: "Something", Status: "True", Severity: "", Reason: "Reason", Message: "message"},
				{Type: "Structural", Status: "False", Severity: "Warning", Reason: "SomethingWrong", Message: "something went wrong"},
			}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Track updated schema
			var updatedSchema *kubebindv1alpha2.BoundAPIResourceSchema
			ctx := context.Background()
			r := &reconciler{
				getCRD:                       tt.getCRD,
				getAPIResourceSchema:         newGetAPIResourceSchema(ctx, tt.schema),
				getBoundAPIResourceSchema:    newGetBoundAPIResourceSchema(ctx, tt.boundSchema),
				updateBoundAPIResourceSchema: newUpdateBoundAPIResourceSchema(&updatedSchema),
			}

			if err := r.ensureCRDConditionsCopiedToBoundSchema(context.Background(), tt.export); (err != nil) != tt.wantErr {
				t.Errorf("ensureCRDConditionsCopiedToBoundSchema() error = %v, wantErr %v", err, tt.wantErr)
			} else if err == nil && updatedSchema != nil {
				for i := range updatedSchema.Status.Conditions {
					updatedSchema.Status.Conditions[i].LastTransitionTime = metav1.Time{}
				}
				for i := range tt.expected.Status.Conditions {
					tt.expected.Status.Conditions[i].LastTransitionTime = metav1.Time{}
				}

				require.Equal(t, tt.expected.Status.Conditions, updatedSchema.Status.Conditions,
					cmp.Diff(tt.expected.Status.Conditions, updatedSchema.Status.Conditions))
			}
		})
	}
}

func newGetCRD(name string, crd *apiextensionsv1.CustomResourceDefinition) func(name string) (*apiextensionsv1.CustomResourceDefinition, error) {
	return func(n string) (*apiextensionsv1.CustomResourceDefinition, error) {
		if n == name {
			return crd, nil
		}
		return nil, errors.NewNotFound(apiextensionsv1.SchemeGroupVersion.WithResource("customresourcedefinitions").GroupResource(), "not found")
	}
}

func newCRD(name string, conditions []apiextensionsv1.CustomResourceDefinitionCondition) *apiextensionsv1.CustomResourceDefinition {
	return &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Status: apiextensionsv1.CustomResourceDefinitionStatus{
			Conditions: conditions,
		},
	}
}

func newAPIResourceSchema(name, namespace, group, plural string) *kubebindv1alpha2.APIResourceSchema {
	return &kubebindv1alpha2.APIResourceSchema{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: kubebindv1alpha2.APIResourceSchemaSpec{
			APIResourceSchemaCRDSpec: kubebindv1alpha2.APIResourceSchemaCRDSpec{
				Group: group,
				Names: apiextensionsv1.CustomResourceDefinitionNames{
					Plural: plural,
				},
			},
		},
	}
}

func newGetAPIResourceSchema(_ context.Context, schema *kubebindv1alpha2.APIResourceSchema) func(ctx context.Context, name string) (*kubebindv1alpha2.APIResourceSchema, error) {
	return func(_ context.Context, name string) (*kubebindv1alpha2.APIResourceSchema, error) {
		if name == schema.Name {
			return schema, nil
		}
		return nil, errors.NewNotFound(kubebindv1alpha2.SchemeGroupVersion.WithResource("apiresourceschemas").GroupResource(), name)
	}
}

func newGetBoundAPIResourceSchema(_ context.Context, boundSchema *kubebindv1alpha2.BoundAPIResourceSchema) func(ctx context.Context, name string) (*kubebindv1alpha2.BoundAPIResourceSchema, error) {
	return func(ctx context.Context, name string) (*kubebindv1alpha2.BoundAPIResourceSchema, error) {
		if name == boundSchema.Name {
			return boundSchema, nil
		}
		return nil, errors.NewNotFound(kubebindv1alpha2.SchemeGroupVersion.WithResource("boundapiresourceschemas").GroupResource(), name)
	}
}

func newUpdateBoundAPIResourceSchema(updatedSchemaPtr **kubebindv1alpha2.BoundAPIResourceSchema) func(context.Context, *kubebindv1alpha2.BoundAPIResourceSchema) (*kubebindv1alpha2.BoundAPIResourceSchema, error) {
	return func(ctx context.Context, boundSchema *kubebindv1alpha2.BoundAPIResourceSchema) (*kubebindv1alpha2.BoundAPIResourceSchema, error) {
		*updatedSchemaPtr = boundSchema.DeepCopy()
		return boundSchema, nil
	}
}

func newExportWithResources(name, namespace string, resources []kubebindv1alpha2.APIResourceSchemaReference) *kubebindv1alpha2.APIServiceExport {
	return &kubebindv1alpha2.APIServiceExport{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: kubebindv1alpha2.APIServiceExportSpec{
			Resources: resources,
		},
	}
}

func newBoundAPIResourceSchema(name string, conditions []conditionsapi.Condition) *kubebindv1alpha2.BoundAPIResourceSchema {
	return &kubebindv1alpha2.BoundAPIResourceSchema{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Status: kubebindv1alpha2.BoundAPIResourceSchemaStatus{
			Conditions: conditions,
		},
	}
}
