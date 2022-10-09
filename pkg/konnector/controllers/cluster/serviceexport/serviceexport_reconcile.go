/*
Copyright 2022 The kube bind Authors.

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

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"

	kubebindv1alpha1 "github.com/kube-bind/kube-bind/pkg/apis/kubebind/v1alpha1"
	conditionsapi "github.com/kube-bind/kube-bind/pkg/apis/third_party/conditions/apis/conditions/v1alpha1"
	"github.com/kube-bind/kube-bind/pkg/apis/third_party/conditions/util/conditions"
)

type reconciler struct {
	listServiceBinding func(export string) ([]*kubebindv1alpha1.ServiceBinding, error)

	updateCRD func(ctx context.Context, crd *apiextensionsv1.CustomResourceDefinition) (*apiextensionsv1.CustomResourceDefinition, error)
	createCRD func(ctx context.Context, crd *apiextensionsv1.CustomResourceDefinition) (*apiextensionsv1.CustomResourceDefinition, error)
}

func (r *reconciler) reconcile(ctx context.Context, export *kubebindv1alpha1.ServiceExport) error {
	var errs []error

	// connected?
	bindings, err := r.listServiceBinding(export.Name)
	if err != nil {
		return err
	}
	if len(bindings) == 0 {
		conditions.MarkFalse(
			export,
			kubebindv1alpha1.ServiceExportConditionConnected,
			"NoServiceBinding",
			conditionsapi.ConditionSeverityInfo,
			"No ServiceBindings found for ServiceExport",
		)
	} else if len(bindings) > 1 {
		conditions.MarkFalse(
			export,
			kubebindv1alpha1.ServiceExportConditionConnected,
			"MultipleServiceBindings",
			conditionsapi.ConditionSeverityError,
			"Multiple ServiceBindings found for ServiceExport. Delete all but one.",
		)
	} else {
		conditions.MarkTrue(
			export,
			kubebindv1alpha1.ServiceExportConditionConnected,
		)

		if err := r.ensureCRDs(ctx, export); err != nil {
			return err
		}
	}

	conditions.SetSummary(export)

	return utilerrors.NewAggregate(errs)
}

func (r *reconciler) ensureCRDs(ctx context.Context, export *kubebindv1alpha1.ServiceExport) error {
	return nil
}
