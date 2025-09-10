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
	"k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kubebindv1alpha2 "github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2"
	"github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2/helpers"
	"github.com/kube-bind/kube-bind/sdk/apis/third_party/conditions/util/conditions"
)

type reconciler struct {
	scope kubebindv1alpha2.InformerScope
}

func (r *reconciler) reconcile(ctx context.Context, cache cache.Cache, client client.Client, export *kubebindv1alpha2.APIServiceExport) error {
	var errs []error
	log := klog.FromContext(ctx)

	if specChanged, err := r.ensureSchema(ctx, cache, export); err != nil {
		errs = append(errs, err)
	} else if specChanged {
		// TODO(mjudeikis): Implement schema lifecycle. Based on how system is configured, we need to either force crd/schema updates
		// or not. This will require a more in-depth analysis of the current system and its requirements. Idea is that we need to watch GVR of schema
		// source and based on some policy setting update boundschemas or not. And propagate them to consumer.
		// https://github.com/kube-bind/kube-bind/issues/301
		log.Info("APIServiceExport schema change detected. Update not implemented.")
	}

	return utilerrors.NewAggregate(errs)
}

func (r *reconciler) ensureSchema(ctx context.Context, cache cache.Cache, export *kubebindv1alpha2.APIServiceExport) (specChanged bool, err error) {
	logger := klog.FromContext(ctx)
	schemas := make([]*kubebindv1alpha2.BoundSchema, 0, len(export.Spec.Resources))

	for _, res := range export.Spec.Resources {
		name := res.Resource + "." + res.Group
		schema, err := r.getBoundSchema(ctx, cache, name)
		if err != nil {
			if errors.IsNotFound(err) {
				continue
			}
			return false, err
		}

		schemas = append(schemas, schema)
	}

	hash := helpers.BoundSchemasSpecHash(schemas)

	if export.Annotations[kubebindv1alpha2.SourceSpecHashAnnotationKey] != hash {
		// both exist, update APIServiceExport
		logger.V(1).Info("Updating APIServiceExport. Hash mismatch", "hash", hash, "expected", export.Annotations[kubebindv1alpha2.SourceSpecHashAnnotationKey])
		if export.Annotations == nil {
			export.Annotations = map[string]string{}
		}
		export.Annotations[kubebindv1alpha2.SourceSpecHashAnnotationKey] = hash
		return true, nil
	}

	conditions.MarkTrue(export, kubebindv1alpha2.APIServiceExportConditionProviderInSync)

	return false, nil
}

func (r *reconciler) getBoundSchema(ctx context.Context, cache cache.Cache, name string) (*kubebindv1alpha2.BoundSchema, error) {
	var schema kubebindv1alpha2.BoundSchema
	key := types.NamespacedName{Name: name}
	if err := cache.Get(ctx, key, &schema); err != nil {
		return nil, err
	}
	return &schema, nil
}
