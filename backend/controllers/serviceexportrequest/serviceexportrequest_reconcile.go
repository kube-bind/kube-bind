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

package serviceexportrequest

import (
	"context"
	"fmt"
	"strings"
	"time"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kube-bind/kube-bind/backend/kubernetes/resources"
	kubebindv1alpha2 "github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2"
	"github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2/helpers"
	conditionsapi "github.com/kube-bind/kube-bind/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
	"github.com/kube-bind/kube-bind/sdk/apis/third_party/conditions/util/conditions"
)

type reconciler struct {
	informerScope          kubebindv1alpha2.InformerScope
	clusterScopedIsolation kubebindv1alpha2.Isolation
	schemaSource           string

	getBoundSchema    func(ctx context.Context, cache cache.Cache, namespace, name string) (*kubebindv1alpha2.BoundSchema, error)
	createBoundSchema func(ctx context.Context, cl client.Client, schema *kubebindv1alpha2.BoundSchema) error

	getServiceExport           func(ctx context.Context, cache cache.Cache, ns, name string) (*kubebindv1alpha2.APIServiceExport, error)
	createServiceExport        func(ctx context.Context, cl client.Client, resource *kubebindv1alpha2.APIServiceExport) error
	deleteServiceExportRequest func(ctx context.Context, cl client.Client, namespace, name string) error
}

func (r *reconciler) reconcile(ctx context.Context, cl client.Client, cache cache.Cache, req *kubebindv1alpha2.APIServiceExportRequest) error {
	// We must ensure schemas are created in form of bound schemas first for validation.
	// Worst case scenario if validation fails, we will reuse schemas for same consumer once issues are fixed.
	if err := r.ensureBoundSchemas(ctx, cl, cache, req); err != nil {
		conditions.SetSummary(req)
		return err
	}

	if err := r.validate(ctx, cl, req); err != nil {
		conditions.SetSummary(req)
		return err
	}

	if err := r.ensureExports(ctx, cl, cache, req); err != nil {
		conditions.SetSummary(req)
		return err
	}

	conditions.SetSummary(req)

	return nil
}

// getExportedSchemas will list all schemas, exported by current backend.
// Important: getExportedSchemas is using client.Client to list resources, not cache.
// This is due to fact we use dynamic client and unstructured.Unstructured to get schemas and it
// does not quite work with dynamic cache informers:
// failed to get informer for *unstructured.UnstructuredList apis.kcp.io/v1alpha1, Kind=APIResourceSchemaList: failed to find newly started informer for apis.kcp.io/v1alpha1, Kind=APIResourceSchema"}.
func (r *reconciler) getExportedSchemas(ctx context.Context, cl client.Client) (kubebindv1alpha2.ExportedSchemas, error) {
	parts := strings.SplitN(r.schemaSource, ".", 3)
	if len(parts) != 3 { // We check this in validation, but just in case.
		return nil, fmt.Errorf("invalid schema source: %q", r.schemaSource)
	}

	gvk := schema.GroupVersionKind{
		Kind:    parts[0],
		Version: parts[1],
		Group:   parts[2],
	}

	// Ensure we have the List kind
	listGVK := gvk
	if !strings.HasSuffix(listGVK.Kind, "List") {
		listGVK.Kind += "List"
	}

	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(listGVK)

	// TODO(mjudeikis): This is hardcoded here and in handlers.go for now.
	labelSelector := labels.Set{
		resources.ExportedCRDsLabel: "true",
	}

	listOpts := []client.ListOption{}
	listOpts = append(listOpts, client.MatchingLabelsSelector{Selector: labelSelector.AsSelector()})

	if err := cl.List(ctx, list, listOpts...); err != nil {
		return nil, err
	}

	var boundSchemas kubebindv1alpha2.ExportedSchemas = make(map[string]*kubebindv1alpha2.BoundSchema, len(list.Items))
	for _, item := range list.Items {
		boundSchema, err := helpers.UnstructuredToBoundSchema(item)
		if err != nil {
			return nil, err
		}
		boundSchemas[boundSchema.ResourceGroupName()] = boundSchema
	}

	return boundSchemas, nil
}

func (r *reconciler) ensureBoundSchemas(ctx context.Context, cl client.Client, cache cache.Cache, req *kubebindv1alpha2.APIServiceExportRequest) error {
	exportedSchemas, err := r.getExportedSchemas(ctx, cl)
	if err != nil {
		return err
	}

	// Ensure all bound schemas exist
	for _, res := range req.Spec.Resources {
		if len(res.Versions) == 0 {
			continue
		}

		for _, boundSchema := range exportedSchemas {
			if boundSchema.Spec.Group == res.Group && boundSchema.Spec.Names.Plural == res.Resource {
				boundSchema.Name = res.ResourceGroupName()
				boundSchema.Namespace = req.Namespace
				boundSchema.Spec.InformerScope = r.informerScope
				boundSchema.ResourceVersion = ""

				obj, err := r.getBoundSchema(ctx, cache, boundSchema.Namespace, boundSchema.Name)
				if err != nil && !apierrors.IsNotFound(err) {
					return err
				}

				// TODO(mjudeikis): https://github.com/kube-bind/kube-bind/issues/297
				if obj != nil {
					continue
				}

				if err := r.createBoundSchema(ctx, cl, boundSchema); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func (r *reconciler) ensureExports(ctx context.Context, cl client.Client, cache cache.Cache, req *kubebindv1alpha2.APIServiceExportRequest) error {
	logger := klog.FromContext(ctx)

	var schemas []*kubebindv1alpha2.BoundSchema
	var scope apiextensionsv1.ResourceScope
	if req.Status.Phase == kubebindv1alpha2.APIServiceExportRequestPhasePending {
		for _, res := range req.Spec.Resources {
			name := res.ResourceGroupName()
			boundSchema, err := r.getBoundSchema(ctx, cache, req.Namespace, name)
			if err != nil {
				if apierrors.IsNotFound(err) {
					conditions.MarkFalse(
						req,
						kubebindv1alpha2.APIServiceExportRequestConditionExportsReady,
						"APIResourceSchemaNotFound",
						conditionsapi.ConditionSeverityError,
						"APIResourceSchema %s in the service provider cluster not found",
						name,
					)
					return err
				}
				return err
			}

			// Collect all schemas for hashing.
			// TODO(mjudeikis) Scope is same for all crds so we keep stamping it over. We might want to change this
			scope = boundSchema.Spec.Scope
			schemas = append(schemas, boundSchema)
		}

		if _, err := r.getServiceExport(ctx, cache, req.Namespace, req.Name); err != nil && !apierrors.IsNotFound(err) {
			return err
		}

		hash := helpers.BoundSchemasSpecHash(schemas)
		export := &kubebindv1alpha2.APIServiceExport{
			ObjectMeta: metav1.ObjectMeta{
				Name:      req.Name,
				Namespace: req.Namespace,
				Annotations: map[string]string{
					kubebindv1alpha2.SourceSpecHashAnnotationKey: hash,
				},
			},
			Spec: kubebindv1alpha2.APIServiceExportSpec{
				InformerScope: r.informerScope,
			},
		}
		if scope == apiextensionsv1.ClusterScoped {
			export.Spec.ClusterScopedIsolation = r.clusterScopedIsolation
		}

		for _, res := range req.Spec.Resources {
			export.Spec.Resources = append(export.Spec.Resources, kubebindv1alpha2.APIServiceExportResource{
				GroupResource: kubebindv1alpha2.GroupResource{
					Group:    res.Group,
					Resource: res.Resource,
				},
				Versions: res.Versions,
			})
		}
		export.Spec.PermissionClaims = req.Spec.PermissionClaims

		logger.V(1).Info("Creating APIServiceExport", "name", export.Name, "namespace", export.Namespace)
		if err := r.createServiceExport(ctx, cl, export); err != nil {
			return err
		}

		conditions.MarkTrue(req, kubebindv1alpha2.APIServiceExportRequestConditionExportsReady)
		req.Status.Phase = kubebindv1alpha2.APIServiceExportRequestPhaseSucceeded

		if time.Since(req.CreationTimestamp.Time) > time.Minute {
			req.Status.Phase = kubebindv1alpha2.APIServiceExportRequestPhaseFailed
			req.Status.TerminalMessage = conditions.GetMessage(req, kubebindv1alpha2.APIServiceExportRequestConditionExportsReady)
		}
	}

	if time.Since(req.CreationTimestamp.Time) > 10*time.Minute {
		logger.Info("Deleting service binding request %s/%s", req.Namespace, req.Name, "reason", "timeout", "age", time.Since(req.CreationTimestamp.Time))
		return r.deleteServiceExportRequest(ctx, cl, req.Namespace, req.Name)
	}

	return nil
}

// Validate validates if the APIServiceExportRequest is in a valid state.
// Currently it validates if all requested schemas are of the same scope and
// if claimable apis are allowed and valid.
func (r *reconciler) validate(ctx context.Context, cl client.Client, req *kubebindv1alpha2.APIServiceExportRequest) error {
	exportedSchemas, err := r.getExportedSchemas(ctx, cl)
	if err != nil {
		return err
	}

	if len(exportedSchemas) == 0 {
		conditions.MarkFalse(
			req,
			kubebindv1alpha2.APIServiceExportRequestConditionExportsReady,
			"SchemaNotFound",
			conditionsapi.ConditionSeverityError,
			"SchemaNotFound not found",
		)
		return fmt.Errorf("no exported schemas found")
	}

	scopes := make([]apiextensionsv1.ResourceScope, 0, len(exportedSchemas))
	for _, res := range req.Spec.Resources {
		boundSchema, ok := exportedSchemas[res.ResourceGroupName()]
		if !ok {
			conditions.MarkFalse(
				req,
				kubebindv1alpha2.APIServiceExportRequestConditionExportsReady,
				"SchemaNotFound",
				conditionsapi.ConditionSeverityError,
				"Schema %s not found",
				res.ResourceGroupName(),
			)
			return fmt.Errorf("schema %s not found", res.ResourceGroupName())
		}
		scopes = append(scopes, boundSchema.Spec.Scope)
	}

	// Check CRD scopes matches.
	if len(scopes) > 1 {
		first := scopes[0]
		for _, scope := range scopes[1:] {
			if scope != first {
				conditions.MarkFalse(req,
					kubebindv1alpha2.APIServiceExportRequestConditionExportsReady,
					"DifferentScopes",
					conditionsapi.ConditionSeverityError,
					"Different scopes found: %v",
					scopes,
				)
				return fmt.Errorf("different scopes found for claimed resources: %v", scopes)
			}
		}
	}

	// Add validation if claimable apis are valid here
	for _, claim := range req.Spec.PermissionClaims {
		if !isClaimableAPI(claim) {
			conditions.MarkFalse(
				req,
				kubebindv1alpha2.APIServiceExportConditionPermissionClaim,
				"InvalidPermissionClaim",
				conditionsapi.ConditionSeverityError,
				"Resource %s is not a valid claimable API",
				claim.GroupResource.String(),
			)
			return fmt.Errorf("resource %s is not a valid claimable API", claim.GroupResource.String())
		}
	}

	return nil
}

// isClaimableAPI checks if a permission claim is for a claimable API.
func isClaimableAPI(claim kubebindv1alpha2.PermissionClaim) bool {
	for _, api := range kubebindv1alpha2.ClaimableAPIs {
		if claim.Group == api.GroupVersionResource.Group && claim.Resource == api.Names.Plural {
			return true
		}
	}
	return false
}
