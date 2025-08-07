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

package servicebinding

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kubebindv1alpha2 "github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2"
	conditionsapi "github.com/kube-bind/kube-bind/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
)

func TestEnsureCRDs(t *testing.T) {
	tests := []struct {
		name             string
		bindingName      string
		getCRD           func(name string) (*apiextensionsv1.CustomResourceDefinition, error)
		boundSchema      *kubebindv1alpha2.BoundAPIResourceSchema
		schema           *kubebindv1alpha2.APIResourceSchema
		serviceExport    *kubebindv1alpha2.APIServiceExport
		expectConditions conditionsapi.Conditions
		wantErr          bool
	}{
		{
			name:        "create-when-crd-missing",
			bindingName: "test-binding",
			getCRD: func(name string) (*apiextensionsv1.CustomResourceDefinition, error) {
				return nil, errors.NewNotFound(apiextensionsv1.SchemeGroupVersion.WithResource("customresourcedefinitions").GroupResource(), name)
			},
			schema:      newAPIResourceSchema("test-schema", "default", "example.com", "tests"),
			boundSchema: newBoundAPIResourceSchema("test-schema", "default", "example.com", "tests"),
			serviceExport: newServiceExportWithResources("test-binding", "default", []kubebindv1alpha2.APIResourceSchemaReference{
				{Name: "test-schema", Type: "APIResourceSchema"},
			}),
			expectConditions: conditionsapi.Conditions{
				conditionsapi.Condition{Type: "Connected", Status: "True"},
				conditionsapi.Condition{Type: "SchemaInSync", Status: "True"},
			},
		},
		{
			name:        "fail-when-external-crd-present",
			bindingName: "test-binding",
			getCRD:      newGetCRD("tests.example.com", newCRD("tests.example.com")),
			schema:      newAPIResourceSchema("test-schema", "default", "example.com", "tests"),
			boundSchema: newBoundAPIResourceSchema("test-schema", "default", "example.com", "tests"),
			serviceExport: newServiceExportWithResources("test-binding", "default", []kubebindv1alpha2.APIResourceSchemaReference{
				{Name: "test-schema", Type: "APIResourceSchema"},
			}),
			expectConditions: conditionsapi.Conditions{
				conditionsapi.Condition{
					Type:     "Connected",
					Status:   "False",
					Severity: "Error",
					Reason:   "ForeignCustomResourceDefinition",
					Message:  "CustomResourceDefinition tests.example.com is not owned by kube-bind.io.",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &reconciler{
				getCRD:           tt.getCRD,
				getServiceExport: newGetServiceExport(tt.serviceExport.Name, tt.serviceExport),
				getAPIResourceSchema: func(ctx context.Context, name string) (*kubebindv1alpha2.APIResourceSchema, error) {
					if name == tt.schema.Name {
						return tt.schema, nil
					}
					return nil, errors.NewNotFound(kubebindv1alpha2.SchemeGroupVersion.WithResource("apiresourceschemas").GroupResource(), name)
				},
				getBoundAPIResourceSchema: func(ctx context.Context, name string) (*kubebindv1alpha2.BoundAPIResourceSchema, error) {
					if name == tt.schema.Name {
						return tt.boundSchema, nil
					}
					return nil, errors.NewNotFound(kubebindv1alpha2.SchemeGroupVersion.WithResource("boundapiresourceschemas").GroupResource(), name)
				},
				createCRD: func(ctx context.Context, crd *apiextensionsv1.CustomResourceDefinition) (*apiextensionsv1.CustomResourceDefinition, error) {
					return crd.DeepCopy(), nil
				},
				updateCRD: func(ctx context.Context, crd *apiextensionsv1.CustomResourceDefinition) (*apiextensionsv1.CustomResourceDefinition, error) {
					return crd.DeepCopy(), nil
				},
				reconcileServiceBinding: func(binding *kubebindv1alpha2.APIServiceBinding) bool {
					return true
				},
			}

			b := newBinding(tt.bindingName)
			err := r.ensureCRDs(context.Background(), b)
			require.NoError(t, err)

			for i := range b.Status.Conditions {
				b.Status.Conditions[i].LastTransitionTime = metav1.Time{} // this is hard to compare
			}
			require.Equal(t, tt.expectConditions, b.Status.Conditions)
		})
	}
}
func newBoundAPIResourceSchema(name, namespace string, group, plural string) *kubebindv1alpha2.BoundAPIResourceSchema {
	return &kubebindv1alpha2.BoundAPIResourceSchema{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: kubebindv1alpha2.BoundAPIResourceSchemaSpec{
			APIResourceSchemaCRDSpec: kubebindv1alpha2.APIResourceSchemaCRDSpec{
				Group: group,
				Names: apiextensionsv1.CustomResourceDefinitionNames{
					Plural: plural,
				},
			},
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

func newServiceExportWithResources(name, namespace string, resources []kubebindv1alpha2.APIResourceSchemaReference) *kubebindv1alpha2.APIServiceExport {
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

func newGetCRD(name string, crd *apiextensionsv1.CustomResourceDefinition) func(name string) (*apiextensionsv1.CustomResourceDefinition, error) {
	return func(n string) (*apiextensionsv1.CustomResourceDefinition, error) {
		if n == name {
			return crd, nil
		}
		return nil, errors.NewNotFound(apiextensionsv1.SchemeGroupVersion.WithResource("customresourcedefinitions").GroupResource(), "not found")
	}
}

func newCRD(name string) *apiextensionsv1.CustomResourceDefinition {
	return &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
}

func newGetServiceExport(name string, crd *kubebindv1alpha2.APIServiceExport) func(name string) (*kubebindv1alpha2.APIServiceExport, error) {
	return func(n string) (*kubebindv1alpha2.APIServiceExport, error) {
		if n == name {
			return crd, nil
		}
		return nil, errors.NewNotFound(kubebindv1alpha2.SchemeGroupVersion.WithResource("apiserviceexports").GroupResource(), "not found")
	}
}

func newBinding(name string) *kubebindv1alpha2.APIServiceBinding {
	return &kubebindv1alpha2.APIServiceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
}
