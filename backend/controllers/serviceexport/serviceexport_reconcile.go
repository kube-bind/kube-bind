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

	"k8s.io/apimachinery/pkg/api/errors"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kubebindv1alpha2 "github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2"
	"github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2/helpers"
	conditionsapi "github.com/kube-bind/kube-bind/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
	"github.com/kube-bind/kube-bind/sdk/apis/third_party/conditions/util/conditions"
)

type reconciler struct {
	getBoundSchema      func(ctx context.Context, cache cache.Cache, namespace, name string) (*kubebindv1alpha2.BoundSchema, error)
	deleteServiceExport func(ctx context.Context, client client.Client, namespace, name string) error
}

func (r *reconciler) reconcile(ctx context.Context, cache cache.Cache, export *kubebindv1alpha2.APIServiceExport) error {
	var errs []error

	if err := r.ensureSchema(ctx, cache, export); err != nil {
		errs = append(errs, err)
	}

	return utilerrors.NewAggregate(errs)
}

func (r *reconciler) ensureSchema(ctx context.Context, cache cache.Cache, export *kubebindv1alpha2.APIServiceExport) error {
	logger := klog.FromContext(ctx)
	schemas := make([]*kubebindv1alpha2.BoundSchema, 0, len(export.Spec.Resources))

	for _, res := range export.Spec.Resources {
		name := res.ResourceGroupName()
		schema, err := r.getBoundSchema(ctx, cache, export.Namespace, name)
		if err != nil {
			if errors.IsNotFound(err) {
				conditions.MarkFalse(
					export,
					kubebindv1alpha2.APIServiceExportConditionProviderInSync,
					"BoundSchemaMissing",
					conditionsapi.ConditionSeverityError,
					"BoundSchema %q is not available: %v", name, err)
				return nil
			}
			return err
		}

		schemas = append(schemas, schema)
	}

	hash, err := helpers.BoundSchemasSpecHash(schemas)
	if err != nil {
		return err
	}

	if export.Annotations[kubebindv1alpha2.SourceSpecHashAnnotationKey] != hash {
		// both exist, update APIServiceExport
		logger.V(1).Info("Updating APIServiceExport. Hash mismatch", "hash", hash, "expected", export.Annotations[kubebindv1alpha2.SourceSpecHashAnnotationKey])
		if export.Annotations == nil {
			export.Annotations = map[string]string{}
		}
		export.Annotations[kubebindv1alpha2.SourceSpecHashAnnotationKey] = hash
		return nil
	}

	conditions.MarkTrue(export, kubebindv1alpha2.APIServiceExportConditionProviderInSync)

	return nil
}
