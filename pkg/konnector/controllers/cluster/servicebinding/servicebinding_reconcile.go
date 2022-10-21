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

package servicebinding

import (
	"context"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/utils/pointer"

	kubebindv1alpha1 "github.com/kube-bind/kube-bind/pkg/apis/kubebind/v1alpha1"
	kubebindhelpers "github.com/kube-bind/kube-bind/pkg/apis/kubebind/v1alpha1/helpers"
	conditionsapi "github.com/kube-bind/kube-bind/pkg/apis/third_party/conditions/apis/conditions/v1alpha1"
	"github.com/kube-bind/kube-bind/pkg/apis/third_party/conditions/util/conditions"
)

type reconciler struct {
	consumerSecretRefKey, providerNamespace string

	getServiceExport  func(ns string) (*kubebindv1alpha1.APIServiceExport, error)
	getServiceBinding func(name string) (*kubebindv1alpha1.APIServiceBinding, error)
	getClusterBinding func(ctx context.Context) (*kubebindv1alpha1.ClusterBinding, error)

	getServiceExportResource          func(name string) (*kubebindv1alpha1.APIServiceExportResource, error)
	updateServiceExportResourceStatus func(ctx context.Context, resource *kubebindv1alpha1.APIServiceExportResource) (*kubebindv1alpha1.APIServiceExportResource, error)

	getCRD    func(name string) (*apiextensionsv1.CustomResourceDefinition, error)
	updateCRD func(ctx context.Context, crd *apiextensionsv1.CustomResourceDefinition) (*apiextensionsv1.CustomResourceDefinition, error)
	createCRD func(ctx context.Context, crd *apiextensionsv1.CustomResourceDefinition) (*apiextensionsv1.CustomResourceDefinition, error)
}

func (r *reconciler) reconcile(ctx context.Context, binding *kubebindv1alpha1.APIServiceBinding) error {
	var errs []error

	if err := r.ensureValidServiceExport(ctx, binding); err != nil {
		errs = append(errs, err)
	}

	if err := r.ensureCRDs(ctx, binding); err != nil {
		errs = append(errs, err)
	}

	if err := r.ensurePrettyName(ctx, binding); err != nil {
		errs = append(errs, err)
	}

	conditions.SetSummary(binding)

	return utilerrors.NewAggregate(errs)
}

func (r *reconciler) ensureValidServiceExport(ctx context.Context, binding *kubebindv1alpha1.APIServiceBinding) error {
	if _, err := r.getServiceExport(binding.Spec.Export); err != nil && !errors.IsNotFound(err) {
		return err
	} else if errors.IsNotFound(err) {
		conditions.MarkFalse(
			binding,
			kubebindv1alpha1.APIServiceBindingConditionConnected,
			"APIServiceExportNotFound",
			conditionsapi.ConditionSeverityError,
			"APIServiceExport %s not found on the service provider cluster. Rerun kubectl bind for repair.",
			binding.Spec.Export,
		)
		return nil
	}

	conditions.MarkTrue(
		binding,
		kubebindv1alpha1.APIServiceBindingConditionConnected,
	)

	return nil
}

func (r *reconciler) ensureCRDs(ctx context.Context, binding *kubebindv1alpha1.APIServiceBinding) error {
	var errs []error

	export, err := r.getServiceExport(binding.Spec.Export)
	if err != nil && !errors.IsNotFound(err) {
		return err
	} else if errors.IsNotFound(err) {
		return nil // nothing we can do here
	}

	resourceValid := true
	schemaInSync := true
nextResource:
	for _, resource := range export.Spec.Resources {
		name := resource.Resource + "." + resource.Group
		resource, err := r.getServiceExportResource(name)
		if err != nil && !errors.IsNotFound(err) {
			errs = append(errs, err)
			continue
		} else if errors.IsNotFound(err) {
			conditions.MarkFalse(
				binding,
				kubebindv1alpha1.APIServiceBindingConditionResourcesValid,
				"ServiceExportResourceNotFound",
				conditionsapi.ConditionSeverityError,
				"APIServiceExportResource %s not found on the service provider cluster.",
				name,
			)
			resourceValid = false
			continue
		}

		if resource.Spec.Scope != apiextensionsv1.NamespaceScoped && export.Spec.Scope != kubebindv1alpha1.ClusterScope {
			conditions.MarkFalse(
				binding,
				kubebindv1alpha1.APIServiceBindingConditionResourcesValid,
				"ServiceExportResourceWrongScope",
				conditionsapi.ConditionSeverityError,
				"APIServiceExportResource %s is Cluster scope, but the APIServiceExport is not.",
				name,
			)
			resourceValid = false
			continue
		}

		crd, err := kubebindhelpers.ServiceExportResourceToCRD(resource)
		if err != nil {
			conditions.MarkFalse(
				binding,
				kubebindv1alpha1.APIServiceBindingConditionResourcesValid,
				"ServiceExportResourceInvalid",
				conditionsapi.ConditionSeverityError,
				"APIServiceExportResource %s on the service provider cluster is invalid: %s",
				name, err,
			)
			resourceValid = false
			continue
		}

		// put binding owner reference on the CRD.
		newReference := metav1.OwnerReference{
			APIVersion: kubebindv1alpha1.SchemeGroupVersion.String(),
			Kind:       "APIServiceBinding",
			Name:       binding.Name,
			UID:        binding.UID,
			Controller: pointer.Bool(true),
		}
		crd.OwnerReferences = append(crd.OwnerReferences, newReference)

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
					binding,
					kubebindv1alpha1.APIServiceExportConditionSchemaInSync,
					"CustomResourceDefinitionCreateFailed",
					conditionsapi.ConditionSeverityError,
					"CustomResourceDefinition %s cannot be created: %s",
					name, err,
				)
				schemaInSync = false
				continue
			}
		} else {
			// first check this really ours and we don't override something else
			foundOther := false
			foundThis := false
			var newOwners []metav1.OwnerReference
			for _, ref := range existing.OwnerReferences {
				parts := strings.SplitN(ref.APIVersion, "/", 2)
				if parts[0] != kubebindv1alpha1.SchemeGroupVersion.Group || ref.Kind != "APIServiceBinding" {
					newOwners = append(newOwners, ref)
					continue
				}

				if ref.Name == binding.Name {
					// found our own reference
					newOwners = append(newOwners, newReference)
					foundThis = true
					continue
				}

				other, err := r.getServiceBinding(ref.Name)
				if err != nil && !errors.IsNotFound(err) {
					errs = append(errs, err)
					continue nextResource
				} else if errors.IsNotFound(err) {
					continue // prune non-existing references
				}

				// we found another binding. Is it of the same service provider?
				if other.Spec.KubeconfigSecretRef == binding.Spec.KubeconfigSecretRef {
					errs = append(errs, err)
					continue
				}

				// here we found a binding from another service provider. So the CRD is not ours.
				conditions.MarkFalse(
					binding,
					kubebindv1alpha1.APIServiceExportConditionSchemaInSync,
					"ForeignCustomResourceDefinition",
					conditionsapi.ConditionSeverityError,
					"CustomResourceDefinition %s is owned by APIServiceBinding %s.",
					name, other.Name,
				)
				schemaInSync = false
				continue nextResource
			}
			if !foundThis && !foundOther {
				// this is not our CRD, we should not touch it
				conditions.MarkFalse(
					binding,
					kubebindv1alpha1.APIServiceExportConditionSchemaInSync,
					"ForeignCustomResourceDefinition",
					conditionsapi.ConditionSeverityError,
					"CustomResourceDefinition %s is not owned by kube-bind.io.",
					name,
				)
				schemaInSync = false
				continue
			}

			// add ourselves as owner if we are not there
			crd.ObjectMeta = existing.ObjectMeta
			if !foundThis {
				newOwners = append(newOwners, newReference)
			}
			crd.ObjectMeta.OwnerReferences = newOwners
			result, err = r.updateCRD(ctx, crd)
			if err != nil {
				errs = append(errs, err)
				continue
			} else if errors.IsInvalid(err) {
				conditions.MarkFalse(
					binding,
					kubebindv1alpha1.APIServiceExportConditionSchemaInSync,
					"CustomResourceDefinitionUpdateFailed",
					conditionsapi.ConditionSeverityError,
					"CustomResourceDefinition %s cannot be updated: %s",
					name, err,
				)
				schemaInSync = false
				continue
			}
		}

		// copy the CRD status onto the APIServiceExportResource
		if result != nil {
			orig := resource
			resource = resource.DeepCopy()
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
			binding,
			kubebindv1alpha1.APIServiceBindingConditionResourcesValid,
		)
	}

	if schemaInSync {
		conditions.MarkTrue(
			binding,
			kubebindv1alpha1.APIServiceBindingConditionSchemaInSync,
		)
	}

	return utilerrors.NewAggregate(errs)
}

func (r *reconciler) ensurePrettyName(ctx context.Context, binding *kubebindv1alpha1.APIServiceBinding) error {
	clusterBinding, err := r.getClusterBinding(ctx)
	if err != nil && !errors.IsNotFound(err) {
		return err
	} else if errors.IsNotFound(err) {
		binding.Status.ProviderPrettyName = ""
		return nil
	}

	binding.Status.ProviderPrettyName = clusterBinding.Spec.ProviderPrettyName

	return nil
}
