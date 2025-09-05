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
	"k8s.io/apimachinery/pkg/api/meta"
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

func (r *reconciler) reconcile(ctx context.Context, mapper meta.RESTMapper, cl client.Client, cache cache.Cache, req *kubebindv1alpha2.APIServiceExportRequest) error {
	if err := r.ensureBoundSchemas(ctx, mapper, cl, cache, req); err != nil {
		return err
	}

	if err := r.ensureExports(ctx, cl, cache, req); err != nil {
		return err
	}

	conditions.SetSummary(req)

	return nil
}

func (r *reconciler) ensureBoundSchemas(ctx context.Context, mapper meta.RESTMapper, cl client.Client, cache cache.Cache, req *kubebindv1alpha2.APIServiceExportRequest) error {
	// Ensure all bound schemas exist
	for _, res := range req.Spec.Resources {
		if len(res.Versions) == 0 {
			continue
		}

		parts := strings.SplitN(r.schemaSource, ".", 3)
		if len(parts) != 3 { // We check this in validation, but just in case.
			return fmt.Errorf("invalid schema source: %q", r.schemaSource)
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
			return err
		}

		for _, item := range list.Items {
			spec := item.Object["spec"]
			if spec == nil {
				return fmt.Errorf("invalid schema %s/%s: no spec found", item.GetNamespace(), item.GetName())
			}
			group := spec.(map[string]interface{})["group"]
			names := spec.(map[string]interface{})["names"]
			if group == nil || names == nil {
				return fmt.Errorf("invalid schema %s/%s: no group or names found", item.GetNamespace(), item.GetName())
			}
			plural := names.(map[string]interface{})["plural"]

			if group == res.Group && plural == res.Resource {
				boundSchema, err := helpers.UnstructuredToBoundSchema(item)
				if err != nil {
					return err
				}
				boundSchema.Name = res.Resource + "." + res.Group
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
			name := res.Resource + "." + res.Group
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
