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

	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"

	kubebindv1alpha1 "github.com/kube-bind/kube-bind/pkg/apis/kubebind/v1alpha1"
	kubebindhelpers "github.com/kube-bind/kube-bind/pkg/apis/kubebind/v1alpha1/helpers"
	conditionsapi "github.com/kube-bind/kube-bind/pkg/apis/third_party/conditions/apis/conditions/v1alpha1"
	"github.com/kube-bind/kube-bind/pkg/apis/third_party/conditions/util/conditions"
)

type reconciler struct {
	listServiceBinding       func(export string) ([]*kubebindv1alpha1.ServiceBinding, error)
	getServiceExportResource func(name string) (*kubebindv1alpha1.ServiceExportResource, error)

	getCRD    func(name string) (*apiextensionsv1.CustomResourceDefinition, error)
	updateCRD func(ctx context.Context, crd *apiextensionsv1.CustomResourceDefinition) (*apiextensionsv1.CustomResourceDefinition, error)
	createCRD func(ctx context.Context, crd *apiextensionsv1.CustomResourceDefinition) (*apiextensionsv1.CustomResourceDefinition, error)

	updateServiceExportResourceStatus func(ctx context.Context, resource *kubebindv1alpha1.ServiceExportResource) (*kubebindv1alpha1.ServiceExportResource, error)
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
	var errs []error

	resourceValid := true
	schemaInSync := true
	for _, resource := range export.Spec.Resources {
		name := resource.Resource + "." + resource.Group
		resource, err := r.getServiceExportResource(name)
		if err != nil && !errors.IsNotFound(err) {
			errs = append(errs, err)
			continue
		} else if errors.IsNotFound(err) {
			conditions.MarkFalse(
				export,
				kubebindv1alpha1.ServiceExportConditionResourcesValid,
				"ServiceExportResourceNotFound",
				conditionsapi.ConditionSeverityError,
				"ServiceExportResource %s not found on the service provider cluster.",
				name,
			)
			resourceValid = false
			continue
		}

		if resource.Spec.Scope != apiextensionsv1.NamespaceScoped && export.Spec.Scope != kubebindv1alpha1.ClusterScope {
			conditions.MarkFalse(
				export,
				kubebindv1alpha1.ServiceExportConditionResourcesValid,
				"ServiceExportResourceWrongScope",
				conditionsapi.ConditionSeverityError,
				"ServiceExportResource %s is Cluster scope, but the ServiceExport is not.",
				name,
			)
			resourceValid = false
			continue
		}

		crd, err := kubebindhelpers.ServiceExportResourceToCRD(resource)
		if err != nil {
			conditions.MarkFalse(
				export,
				kubebindv1alpha1.ServiceExportConditionResourcesValid,
				"ServiceExportResourceInvalid",
				conditionsapi.ConditionSeverityError,
				"ServiceExportResource %s on the service provider cluster is invalid: %s",
				name, err,
			)
			resourceValid = false
			continue
		}

		existing, err := r.getCRD(crd.Name)
		var result *apiextensionsv1.CustomResourceDefinition
		if err != nil && !errors.IsNotFound(err) {
			errs = append(errs, err)
			continue
		} else if errors.IsNotFound(err) {
			result, err = r.createCRD(ctx, crd)
			if err != nil && !errors.IsInvalid(err) {
				errs = append(errs, err)
				continue
			} else if errors.IsInvalid(err) {
				conditions.MarkFalse(
					export,
					kubebindv1alpha1.ServiceExportConditionSchemaInSync,
					"CustomResourceDefinitionCreateFailed",
					conditionsapi.ConditionSeverityError,
					"CustomResourceDefinition %s cannot be created: %s",
					name, err,
				)
				schemaInSync = false
				continue
			}
		} else {
			crd.ObjectMeta = existing.ObjectMeta
			result, err = r.updateCRD(ctx, crd)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			if errors.IsInvalid(err) {
				conditions.MarkFalse(
					export,
					kubebindv1alpha1.ServiceExportConditionSchemaInSync,
					"CustomResourceDefinitionUpdateFailed",
					conditionsapi.ConditionSeverityError,
					"CustomResourceDefinition %s cannot be updated: %s",
					name, err,
				)
				schemaInSync = false
				continue
			}
		}

		// copy the CRD status onto the ServiceExportResource
		if result != nil {
			orig := resource.DeepCopy()
			resource.Status.Conditions = nil
			for _, c := range result.Status.Conditions {
				severity := conditionsapi.ConditionSeverityError
				if c.Status == apiextensionsv1.ConditionTrue {
					severity = conditionsapi.ConditionSeverityNone
				}
				resource.Status.Conditions = append(resource.Status.Conditions, conditionsapi.Condition{
					Type:               conditionsapi.ConditionType(c.Type),
					Status:             corev1.ConditionStatus(c.Status),
					Severity:           severity, // CRD conditions have no severity
					LastTransitionTime: c.LastTransitionTime,
					Reason:             c.Reason,
					Message:            c.Message,
				})
			}
			resource.Status.AcceptedNames = result.Status.AcceptedNames
			resource.Status.StoredVersions = result.Status.StoredVersions
			if !equality.Semantic.DeepEqual(orig, resource) {
				if _, err := r.updateServiceExportResourceStatus(ctx, resource); err != nil {
					errs = append(errs, err)
				}
			}
		}
	}

	if resourceValid {
		conditions.MarkTrue(
			export,
			kubebindv1alpha1.ServiceExportConditionResourcesValid,
		)
	}

	if schemaInSync {
		conditions.MarkTrue(
			export,
			kubebindv1alpha1.ServiceExportConditionSchemaInSync,
		)
	}

	return utilerrors.NewAggregate(errs)
}
