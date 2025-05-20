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

func TestEnsureCRDConditionsCopied(t *testing.T) {
	tests := []struct {
		name     string
		getCRD   func(name string) (*apiextensionsv1.CustomResourceDefinition, error)
		export   *kubebindv1alpha2.APIServiceExport
		expected *kubebindv1alpha2.APIServiceExport
		wantErr  bool
	}{
		{
			name: "merging",
			getCRD: newGetCRD("foo", newCRD("foo", []apiextensionsv1.CustomResourceDefinitionCondition{
				{Type: "Something", Status: "True", Reason: "Reason", Message: "message"},
				{Type: "Established", Status: "True", Reason: "Reason", Message: "message"},
			})),
			export: newExport("foo", []conditionsapi.Condition{
				{Type: "Ready", Status: "False", Severity: "Warning", Reason: "SomethingElseWrong", Message: "something else went wrong"},
				{Type: "Established", Status: "True", Severity: "None", Reason: "Reason", Message: "message"},
				{Type: "Structural", Status: "False", Severity: "Warning", Reason: "SomethingWrong", Message: "something went wrong"},
			}),
			expected: newExport("foo", []conditionsapi.Condition{
				{Type: "Ready", Status: "False", Severity: "Warning", Reason: "SomethingWrong", Message: "something went wrong"},
				{Type: "Established", Status: "True", Severity: "", Reason: "Reason", Message: "message"},
				{Type: "Something", Status: "True", Reason: "Reason", Message: "message"},
				{Type: "Structural", Status: "False", Severity: "Warning", Reason: "SomethingWrong", Message: "something went wrong"},
			}),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &reconciler{
				getCRD: tt.getCRD,
			}
			export := tt.export.DeepCopy()
			if err := r.ensureCRDConditionsCopied(context.Background(), export); (err != nil) != tt.wantErr {
				t.Errorf("ensureCRDConditionsCopied() error = %v, wantErr %v", err, tt.wantErr)
			} else if err == nil {
				for i := range export.Status.Conditions {
					export.Status.Conditions[i].LastTransitionTime = metav1.Time{} // this is hard to compare
				}
				require.Equal(t, tt.expected, export, cmp.Diff(tt.expected, export))
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

func newExport(name string, conditions []conditionsapi.Condition) *kubebindv1alpha2.APIServiceExport {
	return &kubebindv1alpha2.APIServiceExport{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Status: kubebindv1alpha2.APIServiceExportStatus{
			Conditions: conditions,
		},
	}
}
