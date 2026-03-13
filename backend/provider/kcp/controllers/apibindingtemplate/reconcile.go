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

package apibindingtemplate

import (
	"context"
	"fmt"

	apisv1alpha1 "github.com/kcp-dev/sdk/apis/apis/v1alpha1"
	apisv1alpha2 "github.com/kcp-dev/sdk/apis/apis/v1alpha2"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	kubebindv1alpha2 "github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2"
)

// templateNameForBinding derives a deterministic APIServiceExportTemplate name
// from the APIBinding name and scope.
func templateNameForBinding(bindingName string, scope kubebindv1alpha2.InformerScope) string {
	switch scope {
	case kubebindv1alpha2.ClusterScope:
		return bindingName + "-cluster"
	default:
		return bindingName + "-namespaced"
	}
}

// annotationOwnerBinding is the annotation key used to link a template back to
// the APIBinding that owns it.
const annotationOwnerBinding = "apibindingtemplate.kube-bind.io/owner-binding"

type reconciler struct {
	client               client.Client
	scheme               *runtime.Scheme
	getAPIResourceSchema func(ctx context.Context, name string) (*apisv1alpha1.APIResourceSchema, error)
}

func (r *reconciler) reconcile(ctx context.Context, binding *apisv1alpha2.APIBinding) error {
	logger := klog.FromContext(ctx)

	if binding.Status.Phase != apisv1alpha2.APIBindingPhaseBound {
		return nil
	}
	if len(binding.Status.BoundResources) == 0 {
		logger.V(4).Info("APIBinding has no bound resources yet, skipping", "binding", binding.Name)
		return nil
	}

	// Build separate templates per scope.
	templates, err := r.buildTemplates(ctx, binding)
	if err != nil {
		return err
	}

	for _, desired := range templates {
		// Set owner reference: APIBinding owns the template.
		if err := controllerutil.SetOwnerReference(binding, desired, r.scheme); err != nil {
			return fmt.Errorf("failed to set owner reference on APIServiceExportTemplate %q: %w", desired.Name, err)
		}

		if err := r.ensureTemplate(ctx, binding, desired); err != nil {
			return err
		}
	}

	return nil
}

func (r *reconciler) ensureTemplate(ctx context.Context, binding *apisv1alpha2.APIBinding, desired *kubebindv1alpha2.APIServiceExportTemplate) error {
	logger := klog.FromContext(ctx)

	existing := &kubebindv1alpha2.APIServiceExportTemplate{}
	err := r.client.Get(ctx, types.NamespacedName{Name: desired.Name}, existing)
	if errors.IsNotFound(err) {
		logger.Info("Creating APIServiceExportTemplate", "name", desired.Name, "binding", binding.Name)
		if err := r.client.Create(ctx, desired); err != nil {
			return fmt.Errorf("failed to create APIServiceExportTemplate %q: %w", desired.Name, err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to get APIServiceExportTemplate %q: %w", desired.Name, err)
	}

	needsUpdate := false

	if !equality.Semantic.DeepEqual(existing.Spec, desired.Spec) {
		existing.Spec = desired.Spec
		needsUpdate = true
	}

	// Ensure owner reference is set.
	if !metav1.IsControlledBy(existing, binding) {
		if err := controllerutil.SetOwnerReference(binding, existing, r.scheme); err != nil {
			return fmt.Errorf("failed to set owner reference on APIServiceExportTemplate %q: %w", desired.Name, err)
		}
		needsUpdate = true
	}

	if !needsUpdate {
		return nil
	}

	logger.Info("Updating APIServiceExportTemplate", "name", desired.Name, "binding", binding.Name)
	if err := r.client.Update(ctx, existing); err != nil {
		return fmt.Errorf("failed to update APIServiceExportTemplate %q: %w", desired.Name, err)
	}

	return nil
}

// buildTemplates constructs one APIServiceExportTemplate per scope from the
// APIBinding's bound resources. Resources are grouped by scope so that each
// template contains only resources of the same scope (Namespaced or Cluster).
func (r *reconciler) buildTemplates(ctx context.Context, binding *apisv1alpha2.APIBinding) ([]*kubebindv1alpha2.APIServiceExportTemplate, error) {
	type scopedResource struct {
		scope    kubebindv1alpha2.InformerScope
		resource kubebindv1alpha2.APIServiceExportResource
	}

	grouped := make([]scopedResource, 0, len(binding.Status.BoundResources))
	for _, boundRes := range binding.Status.BoundResources {
		schema, err := r.getAPIResourceSchema(ctx, boundRes.Schema.Name)
		if err != nil {
			return nil, fmt.Errorf("failed to get APIResourceSchema %q for binding %q: %w",
				boundRes.Schema.Name, binding.Name, err)
		}

		var versions []string
		for _, v := range schema.Spec.Versions {
			if v.Served {
				versions = append(versions, v.Name)
			}
		}

		scope := kubebindv1alpha2.NamespacedScope
		if schema.Spec.Scope == apiextensionsv1.ClusterScoped {
			scope = kubebindv1alpha2.ClusterScope
		}

		grouped = append(grouped, scopedResource{
			scope: scope,
			resource: kubebindv1alpha2.APIServiceExportResource{
				GroupResource: kubebindv1alpha2.GroupResource{
					Group:    boundRes.Group,
					Resource: boundRes.Resource,
				},
				Versions: versions,
			},
		})
	}

	// Group resources by scope.
	byScope := map[kubebindv1alpha2.InformerScope][]kubebindv1alpha2.APIServiceExportResource{}
	for _, sr := range grouped {
		byScope[sr.scope] = append(byScope[sr.scope], sr.resource)
	}

	templates := make([]*kubebindv1alpha2.APIServiceExportTemplate, 0, len(byScope))
	for scope, resources := range byScope {
		templateName := templateNameForBinding(binding.Name, scope)
		tmpl := &kubebindv1alpha2.APIServiceExportTemplate{
			ObjectMeta: metav1.ObjectMeta{
				Name: templateName,
				Annotations: map[string]string{
					annotationOwnerBinding: binding.Name,
				},
			},
			Spec: kubebindv1alpha2.APIServiceExportTemplateSpec{
				Scope:     scope,
				Resources: resources,
			},
		}
		templates = append(templates, tmpl)
	}

	return templates, nil
}
