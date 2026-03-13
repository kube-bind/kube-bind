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

// Package apiresourceschema contains a kcp-specific controller that watches
// APIServiceExportTemplates and copies the corresponding APIResourceSchemas
// into the workspace so that the serviceexportrequest controller can find them.
package apiresourceschema

import (
	"context"
	"fmt"

	apisv1alpha2 "github.com/kcp-dev/sdk/apis/apis/v1alpha2"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"
	mcbuilder "sigs.k8s.io/multicluster-runtime/pkg/builder"
	mcmanager "sigs.k8s.io/multicluster-runtime/pkg/manager"
	mcreconcile "sigs.k8s.io/multicluster-runtime/pkg/reconcile"

	"github.com/kube-bind/kube-bind/backend/provider/kcp/controllers/shared"
	kubebindv1alpha2 "github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2"
)

const controllerName = "kube-bind-kcp-apiresourceschema"

// APIResourceSchemaReconciler watches APIServiceExportTemplates and ensures
// that the APIResourceSchemas referenced by the owning APIBinding are copied
// into the workspace with the kube-bind.io/exported=true label.
type APIResourceSchemaReconciler struct {
	manager mcmanager.Manager
	opts    controller.TypedOptions[mcreconcile.Request]
	scheme  *runtime.Scheme
	vwCache *shared.VWClientCache
}

// New returns a new APIResourceSchemaReconciler.
func New(
	ctx context.Context,
	mgr mcmanager.Manager,
	opts controller.TypedOptions[mcreconcile.Request],
	baseConfig *rest.Config,
	scheme *runtime.Scheme,
) (*APIResourceSchemaReconciler, error) {
	return &APIResourceSchemaReconciler{
		manager: mgr,
		opts:    opts,
		scheme:  scheme,
		vwCache: shared.NewVWClientCache(baseConfig, scheme),
	}, nil
}

// Reconcile implements reconcile.Reconciler for multicluster-runtime.
func (r *APIResourceSchemaReconciler) Reconcile(ctx context.Context, req mcreconcile.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling APIServiceExportTemplate for schema copy", "request", req)

	cl, err := r.manager.GetCluster(ctx, req.ClusterName)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get cluster %q: %w", req.ClusterName, err)
	}

	c := cl.GetClient()
	clusterConfig := cl.GetConfig()

	// Get the template.
	tmpl := &kubebindv1alpha2.APIServiceExportTemplate{}
	if err := c.Get(ctx, req.NamespacedName, tmpl); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("failed to get APIServiceExportTemplate %q: %w", req.Name, err)
	}

	// Find the owning APIBinding via annotation.
	bindingName, ok := tmpl.Annotations[shared.AnnotationOwnerBinding]
	if !ok {
		logger.V(4).Info("Template has no owner-binding annotation, skipping", "name", tmpl.Name)
		return ctrl.Result{}, nil
	}

	binding := &apisv1alpha2.APIBinding{}
	if err := c.Get(ctx, types.NamespacedName{Name: bindingName}, binding); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Owning APIBinding not found, skipping", "binding", bindingName)
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("failed to get APIBinding %q: %w", bindingName, err)
	}

	if binding.Status.Phase != apisv1alpha2.APIBindingPhaseBound {
		return ctrl.Result{}, nil
	}

	// Build schema getter with VW fallback using the shared cache.
	getSchema := shared.SchemaGetterWithFallback(c, clusterConfig, r.vwCache)

	rec := reconciler{
		client:               c,
		scheme:               r.scheme,
		getAPIResourceSchema: getSchema,
	}

	if err := rec.reconcile(ctx, tmpl, binding); err != nil {
		logger.Error(err, "Failed to reconcile schemas for template", "name", tmpl.Name)
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager registers the controller with the multicluster-runtime Manager.
func (r *APIResourceSchemaReconciler) SetupWithManager(mgr mcmanager.Manager) error {
	return mcbuilder.ControllerManagedBy(mgr).
		For(&kubebindv1alpha2.APIServiceExportTemplate{}).
		WithOptions(r.opts).
		Named(controllerName).
		Complete(r)
}
