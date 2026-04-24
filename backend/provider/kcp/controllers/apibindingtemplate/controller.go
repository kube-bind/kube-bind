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

// Package apibindingtemplate contains a kcp-specific controller that watches
// APIBindings in provider/backend workspaces and automatically creates or
// updates APIServiceExportTemplates based on the APIResourceSchemas exposed by
// each bound APIExport.
package apibindingtemplate

import (
	"context"
	"fmt"
	"strings"

	apisv1alpha2 "github.com/kcp-dev/sdk/apis/apis/v1alpha2"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	crhandler "sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	mcbuilder "sigs.k8s.io/multicluster-runtime/pkg/builder"
	mchandler "sigs.k8s.io/multicluster-runtime/pkg/handler"
	mcmanager "sigs.k8s.io/multicluster-runtime/pkg/manager"
	"sigs.k8s.io/multicluster-runtime/pkg/multicluster"
	mcreconcile "sigs.k8s.io/multicluster-runtime/pkg/reconcile"

	"github.com/kube-bind/kube-bind/backend/provider/kcp/controllers/shared"
	kubebindv1alpha2 "github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2"
)

const controllerName = "kube-bind-kcp-apibinding-template"

// APIBindingTemplateReconciler watches APIBindings and ensures an
// APIServiceExportTemplate exists for every bound APIExport.
type APIBindingTemplateReconciler struct {
	manager        mcmanager.Manager
	opts           controller.TypedOptions[mcreconcile.Request]
	ignorePrefixes []string
	scheme         *runtime.Scheme
	vwCache        *shared.VWClientCache
}

// New returns a new APIBindingTemplateReconciler.
func New(
	ctx context.Context,
	mgr mcmanager.Manager,
	opts controller.TypedOptions[mcreconcile.Request],
	ignorePrefixes []string,
	baseConfig *rest.Config,
	scheme *runtime.Scheme,
) (*APIBindingTemplateReconciler, error) {
	r := &APIBindingTemplateReconciler{
		manager:        mgr,
		opts:           opts,
		ignorePrefixes: ignorePrefixes,
		scheme:         scheme,
		vwCache:        shared.NewVWClientCache(baseConfig, scheme),
	}

	return r, nil
}

// shouldIgnore returns true if the APIBinding name matches any of the
// configured ignore prefixes.
func (r *APIBindingTemplateReconciler) shouldIgnore(name string) bool {
	for _, prefix := range r.ignorePrefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

// Reconcile implements reconcile.Reconciler for multicluster-runtime.
func (r *APIBindingTemplateReconciler) Reconcile(ctx context.Context, req mcreconcile.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	if r.shouldIgnore(req.Name) {
		logger.V(4).Info("Ignoring APIBinding matching ignore prefix", "name", req.Name)
		return ctrl.Result{}, nil
	}

	logger.Info("Reconciling APIBinding", "request", req)

	cl, err := r.manager.GetCluster(ctx, req.ClusterName)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get client for cluster %q: %w", req.ClusterName, err)
	}

	c := cl.GetClient()
	clusterConfig := cl.GetConfig()

	binding := &apisv1alpha2.APIBinding{}
	if err := c.Get(ctx, req.NamespacedName, binding); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("failed to get APIBinding %q: %w", req.Name, err)
	}

	// Build the schema getter with VW fallback using the shared cache.
	getSchema := shared.SchemaGetterWithFallback(c, clusterConfig, r.vwCache)

	rec := reconciler{
		client:               c,
		scheme:               r.scheme,
		getAPIResourceSchema: getSchema,
	}

	if err := rec.reconcile(ctx, binding); err != nil {
		logger.Error(err, "Failed to reconcile APIBinding", "name", req.Name)
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// getTemplateMapper returns a mapper that enqueues the owning APIBinding when
// an APIServiceExportTemplate changes.
//
// This function has the signature func(clusterName string, cl cluster.Cluster) handler.TypedEventHandler
// because multicluster-runtime's mcbuilder.Watches accepts a "per-cluster event handler factory"
// rather than a plain handler — it calls this factory for each cluster that is engaged.
func getTemplateMapper(clusterName multicluster.ClusterName, cl cluster.Cluster) crhandler.TypedEventHandler[client.Object, mcreconcile.Request] {
	return mchandler.TypedEnqueueRequestsFromMapFuncWithClusterPreservation(func(ctx context.Context, obj client.Object) []mcreconcile.Request {
		annotations := obj.GetAnnotations()
		if annotations == nil {
			return nil
		}
		ownerName, ok := annotations[shared.AnnotationOwnerBinding]
		if !ok {
			return nil
		}

		c := cl.GetClient()
		var binding apisv1alpha2.APIBinding
		if err := c.Get(ctx, client.ObjectKey{Name: ownerName}, &binding); err != nil {
			return nil
		}

		return []mcreconcile.Request{
			{
				Request: reconcile.Request{
					NamespacedName: types.NamespacedName{Name: ownerName},
				},
				ClusterName: clusterName,
			},
		}
	})
}

// SetupWithManager registers the controller with the multicluster-runtime Manager.
func (r *APIBindingTemplateReconciler) SetupWithManager(mgr mcmanager.Manager) error {
	return mcbuilder.ControllerManagedBy(mgr).
		For(&apisv1alpha2.APIBinding{}).
		Watches(
			&kubebindv1alpha2.APIServiceExportTemplate{},
			getTemplateMapper,
		).
		WithOptions(r.opts).
		Named(controllerName).
		Complete(r)
}
