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
	"fmt"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/utils/ptr"

	kubebindv1alpha2 "github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2"
	kubebindhelpers "github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2/helpers"
	conditionsapi "github.com/kube-bind/kube-bind/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
	"github.com/kube-bind/kube-bind/sdk/apis/third_party/conditions/util/conditions"
)

type reconciler struct {
	consumerSecretRefKey, providerNamespace string

	reconcileServiceBinding func(binding *kubebindv1alpha2.APIServiceBinding) bool
	getServiceExport        func(name string) (*kubebindv1alpha2.APIServiceExport, error)
	getServiceExportRequest func(name string) (*kubebindv1alpha2.APIServiceExportRequest, error)
	getServiceBinding       func(name string) (*kubebindv1alpha2.APIServiceBinding, error)
	getClusterBinding       func(ctx context.Context) (*kubebindv1alpha2.ClusterBinding, error)
	getBoundSchema          func(ctx context.Context, name string) (*kubebindv1alpha2.BoundSchema, error)

	updateServiceExportStatus func(ctx context.Context, export *kubebindv1alpha2.APIServiceExport) (*kubebindv1alpha2.APIServiceExport, error)

	getCRD    func(name string) (*apiextensionsv1.CustomResourceDefinition, error)
	updateCRD func(ctx context.Context, crd *apiextensionsv1.CustomResourceDefinition) (*apiextensionsv1.CustomResourceDefinition, error)
	createCRD func(ctx context.Context, crd *apiextensionsv1.CustomResourceDefinition) (*apiextensionsv1.CustomResourceDefinition, error)
}

func (r *reconciler) reconcile(ctx context.Context, binding *kubebindv1alpha2.APIServiceBinding) error {
	var errs []error

	// As konnector is running APIServiceBinding controller for each provider cluster,
	// so each controller should skip others provider's APIServiceBinding object
	if !r.reconcileServiceBinding(binding) {
		return nil
	}

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

func (r *reconciler) ensureValidServiceExport(_ context.Context, binding *kubebindv1alpha2.APIServiceBinding) error {
	if _, err := r.getServiceExport(binding.Name); err != nil && !errors.IsNotFound(err) {
		return err
	} else if errors.IsNotFound(err) {
		// If serviceexport is not found, check if request is not in failed state already:
		request, errRequest := r.getServiceExportRequest(binding.Name)
		if errRequest == nil && request.Status.Phase == kubebindv1alpha2.APIServiceExportRequestPhaseFailed {
			// If request is in failed state, propagate the message to binding status and mark as failed
			conditions.MarkFalse(
				binding,
				kubebindv1alpha2.APIServiceBindingConditionConnected,
				"APIServiceExportFailed",
				conditionsapi.ConditionSeverityError,
				request.Status.TerminalMessage,
				binding.Name,
			)
			return fmt.Errorf("APIServiceExportRequest %s is in failed state: %s", binding.Name, request.Status.TerminalMessage)
		}

		conditions.MarkFalse(
			binding,
			kubebindv1alpha2.APIServiceBindingConditionConnected,
			"APIServiceExportNotFound",
			conditionsapi.ConditionSeverityError,
			"APIServiceExport %s not found on the service provider cluster. Rerun kubectl bind for repair.",
			binding.Name,
		)
		return nil
	}

	conditions.MarkTrue(
		binding,
		kubebindv1alpha2.APIServiceBindingConditionConnected,
	)

	return nil
}

func (r *reconciler) ensureCRDs(ctx context.Context, binding *kubebindv1alpha2.APIServiceBinding) error {
	var errs []error

	export, err := r.getServiceExport(binding.Name)
	if err != nil && !errors.IsNotFound(err) {
		return err
	} else if errors.IsNotFound(err) {
		conditions.MarkFalse(
			binding,
			kubebindv1alpha2.APIServiceBindingConditionConnected,
			"APIServiceExportNotFound",
			conditionsapi.ConditionSeverityError,
			"APIServiceExport %s not found on the service provider cluster.",
			binding.Name,
		)
		return nil // nothing we can do here
	}

	// Get all BoundSchema objects referenced by the export
	schemas, err := r.getSchemasFromExport(ctx, export)
	if err != nil {
		conditions.MarkFalse(
			binding,
			kubebindv1alpha2.APIServiceBindingConditionConnected,
			"BoundSchemaFetchFailed",
			conditionsapi.ConditionSeverityError,
			"Failed to fetch BoundSchema objects: %s",
			err,
		)
		// We dont have schema - try again. Might be a race on provider side.
		return err
	}

	// Process each schema
	for _, schema := range schemas {
		if err := r.referenceBoundSchema(ctx, binding, schema.Name); err != nil {
			errs = append(errs, err)
		}

		if err := r.ensureCRDsFromBoundSchema(ctx, binding, schema); err != nil {
			errs = append(errs, err)
		}
	}

	// If we processed all schemas without errors, mark the binding as connected
	if len(errs) == 0 && !conditions.IsFalse(binding, kubebindv1alpha2.APIServiceBindingConditionConnected) {
		conditions.MarkTrue(binding, kubebindv1alpha2.APIServiceBindingConditionConnected)
		conditions.MarkTrue(binding, kubebindv1alpha2.APIServiceBindingConditionSchemaInSync)
	}

	return utilerrors.NewAggregate(errs)
}

func (r *reconciler) referenceBoundSchema(ctx context.Context, binding *kubebindv1alpha2.APIServiceBinding, name string) error {
	boundSchema, err := r.getBoundSchema(ctx, name)
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to get BoundSchema %s: %w", name, err)
	}

	if boundSchema == nil {
		return nil
	}

	group := boundSchema.Spec.Group
	resource := boundSchema.Spec.Names.Plural

	if len(binding.Status.BoundSchemas) > 0 {
		for _, ref := range binding.Status.BoundSchemas {
			if ref.Group == group && ref.Resource == resource {
				return nil
			}
		}
	}

	if binding.Status.BoundSchemas == nil {
		binding.Status.BoundSchemas = []kubebindv1alpha2.BoundSchemaReference{}
	}

	binding.Status.BoundSchemas = append(binding.Status.BoundSchemas,
		kubebindv1alpha2.BoundSchemaReference{
			GroupResource: kubebindv1alpha2.GroupResource{
				Group:    group,
				Resource: resource,
			},
		})

	return nil
}

func (r *reconciler) ensureCRDsFromBoundSchema(ctx context.Context, binding *kubebindv1alpha2.APIServiceBinding, schema *kubebindv1alpha2.BoundSchema) error {
	var errs []error
	crd := kubebindhelpers.BoundSchemaToCRD(schema)

	newReference := metav1.OwnerReference{
		APIVersion: kubebindv1alpha2.SchemeGroupVersion.String(),
		Kind:       "APIServiceBinding",
		Name:       binding.Name,
		UID:        binding.UID,
		Controller: ptr.To(true),
	}
	crd.OwnerReferences = append(crd.OwnerReferences, newReference)

	existing, err := r.getCRD(crd.Name)
	if err != nil && !errors.IsNotFound(err) {
		return err
	} else if errors.IsNotFound(err) {
		if _, err := r.createCRD(ctx, crd); err != nil && !errors.IsInvalid(err) {
			return err
		} else if errors.IsInvalid(err) {
			conditions.MarkFalse(
				binding,
				kubebindv1alpha2.APIServiceBindingConditionConnected,
				"CustomResourceDefinitionCreateFailed",
				conditionsapi.ConditionSeverityError,
				"CustomResourceDefinition %s cannot be created: %s",
				crd.Name, err,
			)
			return nil
		}

		return nil
	}
	if !kubebindhelpers.IsOwnedByBinding(binding.Name, binding.UID, existing.OwnerReferences) {
		conditions.MarkFalse(
			binding,
			kubebindv1alpha2.APIServiceBindingConditionConnected,
			"ForeignCustomResourceDefinition",
			conditionsapi.ConditionSeverityError,
			"CustomResourceDefinition %s is not owned by kube-bind.io.",
			crd.Name,
		)
		return nil
	}

	crd.ObjectMeta = existing.ObjectMeta
	if _, err := r.updateCRD(ctx, crd); err != nil && !errors.IsInvalid(err) {
		return nil
	} else if errors.IsInvalid(err) {
		conditions.MarkFalse(
			binding,
			kubebindv1alpha2.APIServiceBindingConditionConnected,
			"CustomResourceDefinitionUpdateFailed",
			conditionsapi.ConditionSeverityError,
			"CustomResourceDefinition %s cannot be updated: %s",
			binding.Name, err,
		)
		return nil
	}

	return utilerrors.NewAggregate(errs)
}

func (r *reconciler) ensurePrettyName(ctx context.Context, binding *kubebindv1alpha2.APIServiceBinding) error {
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

// getSchemasFromExport will return all schemas, based on export we are dealing with.
func (r *reconciler) getSchemasFromExport(ctx context.Context, export *kubebindv1alpha2.APIServiceExport) ([]*kubebindv1alpha2.BoundSchema, error) {
	schemas := make([]*kubebindv1alpha2.BoundSchema, 0, len(export.Spec.Resources))

	for _, res := range export.Spec.Resources {
		schema, err := r.getBoundSchema(ctx, res.ResourceGroupName())
		if err != nil {
			return nil, fmt.Errorf("failed to get Schema %s: %w",
				res.ResourceGroupName(), err)
		}

		schemas = append(schemas, schema)
	}

	return schemas, nil
}
