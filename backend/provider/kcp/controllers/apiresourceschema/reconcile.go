/*
Copyright 2026 The Kube Bind Authors.

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

package apiresourceschema

import (
	"context"
	"fmt"

	apisv1alpha1 "github.com/kcp-dev/sdk/apis/apis/v1alpha1"
	apisv1alpha2 "github.com/kcp-dev/sdk/apis/apis/v1alpha2"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/kube-bind/kube-bind/backend/kubernetes/resources"
	kubebindv1alpha2 "github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2"
)

type reconciler struct {
	client               client.Client
	scheme               *runtime.Scheme
	getAPIResourceSchema func(ctx context.Context, name string) (*apisv1alpha1.APIResourceSchema, error)
}

func (r *reconciler) reconcile(ctx context.Context, tmpl *kubebindv1alpha2.APIServiceExportTemplate, binding *apisv1alpha2.APIBinding) error {
	logger := klog.FromContext(ctx)

	// Build a set of resources listed in this template so we only copy
	// schemas that match this template's scope.
	templateResources := make(map[string]bool, len(tmpl.Spec.Resources))
	for _, res := range tmpl.Spec.Resources {
		templateResources[res.Group+"/"+res.Resource] = true
	}

	for _, boundRes := range binding.Status.BoundResources {
		if !templateResources[boundRes.Group+"/"+boundRes.Resource] {
			continue
		}

		schema, err := r.getAPIResourceSchema(ctx, boundRes.Schema.Name)
		if err != nil {
			return fmt.Errorf("failed to get APIResourceSchema %q for binding %q: %w",
				boundRes.Schema.Name, binding.Name, err)
		}

		if err := r.ensureSchema(ctx, tmpl, schema); err != nil {
			logger.Error(err, "Failed to ensure APIResourceSchema copy",
				"schema", schema.Name, "template", tmpl.Name)
			return err
		}
	}

	return nil
}

// ensureSchema creates or updates a local copy of the APIResourceSchema,
// owned by the template and labeled for discovery by serviceexportrequest.
func (r *reconciler) ensureSchema(ctx context.Context, tmpl *kubebindv1alpha2.APIServiceExportTemplate, source *apisv1alpha1.APIResourceSchema) error {
	logger := klog.FromContext(ctx)

	desired := &apisv1alpha1.APIResourceSchema{
		ObjectMeta: metav1.ObjectMeta{
			Name: source.Name,
			Labels: map[string]string{
				resources.ExportedCRDsLabel: "true",
			},
		},
		Spec: *source.Spec.DeepCopy(),
	}

	// Set owner reference: template owns the schema copy.
	if err := controllerutil.SetOwnerReference(tmpl, desired, r.scheme); err != nil {
		return fmt.Errorf("failed to set owner reference on APIResourceSchema %q: %w", desired.Name, err)
	}

	existing := &apisv1alpha1.APIResourceSchema{}
	err := r.client.Get(ctx, client.ObjectKey{Name: desired.Name}, existing)
	if errors.IsNotFound(err) {
		logger.Info("Creating APIResourceSchema copy", "name", desired.Name, "template", tmpl.Name)
		if err := r.client.Create(ctx, desired); err != nil {
			return fmt.Errorf("failed to create APIResourceSchema %q: %w", desired.Name, err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to get APIResourceSchema %q: %w", desired.Name, err)
	}

	// Check if spec or labels need updating.
	needsUpdate := false

	if !equality.Semantic.DeepEqual(existing.Spec, desired.Spec) {
		existing.Spec = desired.Spec
		needsUpdate = true
	}

	if existing.Labels == nil || existing.Labels[resources.ExportedCRDsLabel] != "true" {
		if existing.Labels == nil {
			existing.Labels = make(map[string]string)
		}
		existing.Labels[resources.ExportedCRDsLabel] = "true"
		needsUpdate = true
	}

	// Ensure owner reference is set.
	if !metav1.IsControlledBy(existing, tmpl) {
		if err := controllerutil.SetOwnerReference(tmpl, existing, r.scheme); err != nil {
			return fmt.Errorf("failed to set owner reference on APIResourceSchema %q: %w", existing.Name, err)
		}
		needsUpdate = true
	}

	if needsUpdate {
		logger.Info("Updating APIResourceSchema copy", "name", existing.Name, "template", tmpl.Name)
		if err := r.client.Update(ctx, existing); err != nil {
			return fmt.Errorf("failed to update APIResourceSchema %q: %w", existing.Name, err)
		}
	}

	return nil
}
