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

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"

	kubebindv1alpha2 "github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2"
	kubebindhelpers "github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2/helpers"
	conditionsapi "github.com/kube-bind/kube-bind/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
	"github.com/kube-bind/kube-bind/sdk/apis/third_party/conditions/util/conditions"
)

type reconciler struct {
	getCRD               func(name string) (*apiextensionsv1.CustomResourceDefinition, error)
	getAPIResourceSchema func(ctx context.Context, name string) (*kubebindv1alpha2.APIResourceSchema, error)
	deleteServiceExport  func(ctx context.Context, namespace, name string) error

	requeue func(export *kubebindv1alpha2.APIServiceExport)
}

func (r *reconciler) reconcile(ctx context.Context, export *kubebindv1alpha2.APIServiceExport) error {
	var errs []error

	if specChanged, err := r.ensureSchema(ctx, export); err != nil {
		errs = append(errs, err)
	} else if specChanged {
		// TODO: This should be separate controller for apiresourceschemas.
		// This is wrong place now.
		//	r.requeue(export)
		return nil
	}

	return utilerrors.NewAggregate(errs)
}

func (r *reconciler) ensureSchema(ctx context.Context, export *kubebindv1alpha2.APIServiceExport) (specChanged bool, err error) {
	logger := klog.FromContext(ctx)

	for _, resourceRef := range export.Spec.Resources {
		if resourceRef.Type != "APIResourceSchema" {
			logger.V(1).Info("Skipping unsupported resource type", "type", resourceRef.Type)
			continue
		}

		schema, err := r.getAPIResourceSchema(ctx, resourceRef.Name)
		if err != nil {
			if errors.IsNotFound(err) {
				continue
			}
			return false, err
		}
		crd, err := r.getCRD(schema.Name)
		if err != nil && !errors.IsNotFound(err) {
			return false, err
		}

		if crd == nil {
			conditions.MarkFalse(
				export,
				kubebindv1alpha2.APIServiceExportConditionProviderInSync,
				"CustomResourceDefinitionMissing",
				conditionsapi.ConditionSeverityError,
				"CustomResourceDefinition %s is missing",
				export.Name,
			)
		} else {
			expected, err := kubebindhelpers.CRDToAPIResourceSchema(crd, "")
			if err != nil {
				conditions.MarkFalse(
					export,
					kubebindv1alpha2.APIServiceExportConditionProviderInSync,
					"CustomResourceDefinitionUpdateFailed",
					conditionsapi.ConditionSeverityError,
					"CustomResourceDefinition %s cannot be converted into a APIServiceExport: %s",
					export.Name, err,
				)
				return false, nil // nothing we can do
			}
			hash := kubebindhelpers.APIResourceSchemaCRDSpecHash(&expected.Spec.APIResourceSchemaCRDSpec)
			if export.Annotations[kubebindv1alpha2.SourceSpecHashAnnotationKey] != hash {
				// both exist, update APIServiceExport
				logger.V(1).Info("Updating APIServiceExport. Hash mismatch", "hash", hash, "expected", export.Annotations[kubebindv1alpha2.SourceSpecHashAnnotationKey])
				schema.Spec.APIResourceSchemaCRDSpec = expected.Spec.APIResourceSchemaCRDSpec
				if schema.Annotations == nil {
					schema.Annotations = map[string]string{}
				}
				schema.Annotations[kubebindv1alpha2.SourceSpecHashAnnotationKey] = hash
				return true, nil
			}

			conditions.MarkTrue(export, kubebindv1alpha2.APIServiceExportConditionProviderInSync)
		}
	}

	return false, nil
}
