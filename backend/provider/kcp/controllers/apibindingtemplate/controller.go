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
	"net/url"
	"strings"

	apisv1alpha1 "github.com/kcp-dev/sdk/apis/apis/v1alpha1"
	apisv1alpha2 "github.com/kcp-dev/sdk/apis/apis/v1alpha2"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	mcbuilder "sigs.k8s.io/multicluster-runtime/pkg/builder"
	mcmanager "sigs.k8s.io/multicluster-runtime/pkg/manager"
	mcreconcile "sigs.k8s.io/multicluster-runtime/pkg/reconcile"

	kubebindv1alpha2 "github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2"
)

const controllerName = "kube-bind-kcp-apibinding-template"

// APIBindingTemplateReconciler watches APIBindings and ensures an
// APIServiceExportTemplate exists for every bound APIExport.
type APIBindingTemplateReconciler struct {
	manager        mcmanager.Manager
	opts           controller.TypedOptions[mcreconcile.Request]
	ignorePrefixes []string

	// baseConfig is the kcp admin/root REST config used to construct
	// VW clients for the apiresourceschema virtual workspace.
	baseConfig *rest.Config
	scheme     *runtime.Scheme
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
		baseConfig:     baseConfig,
		scheme:         scheme,
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

// extractClusterID extracts the cluster ID from an apiexport virtual workspace
// URL. The URL format is:
// https://host:port/services/apiexport/root:org:ws/<apiexport-name>/clusters/{cluster-id}
func extractClusterID(clusterConfig *rest.Config) (string, error) {
	u, err := url.Parse(clusterConfig.Host)
	if err != nil {
		return "", fmt.Errorf("failed to parse cluster host URL: %w", err)
	}

	pathParts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(pathParts) < 6 || pathParts[4] != "clusters" {
		return "", fmt.Errorf("unexpected apiexport URL format: %s", u.Path)
	}

	return pathParts[5], nil
}

// newVWClient creates a client pointing at the apiresourceschema virtual workspace
// for the given cluster ID:
// https://host:port/services/apiresourceschema/{clusterID}/clusters/*/
func (r *APIBindingTemplateReconciler) newVWClient(clusterID string) (client.Client, error) {
	cfg := rest.CopyConfig(r.baseConfig)
	u, err := url.Parse(cfg.Host)
	if err != nil {
		return nil, fmt.Errorf("failed to parse base config host: %w", err)
	}

	u.Path = fmt.Sprintf("/services/apiresourceschema/%s/clusters/*", clusterID)
	cfg.Host = u.String()

	return client.New(cfg, client.Options{Scheme: r.scheme})
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

	// Build the schema getter with VW fallback.
	getSchema := r.schemaGetterWithFallback(c, clusterConfig)

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

// schemaGetterWithFallback returns a function that first tries to get the
// APIResourceSchema from the current workspace, and if not found, falls back
// to the apiresourceschema virtual workspace.
func (r *APIBindingTemplateReconciler) schemaGetterWithFallback(
	workspaceClient client.Client,
	clusterConfig *rest.Config,
) func(ctx context.Context, name string) (*apisv1alpha1.APIResourceSchema, error) {
	return func(ctx context.Context, name string) (*apisv1alpha1.APIResourceSchema, error) {
		logger := log.FromContext(ctx)

		// 1. Try the current workspace first.
		var schema apisv1alpha1.APIResourceSchema
		err := workspaceClient.Get(ctx, client.ObjectKey{Name: name}, &schema)
		if err == nil {
			return &schema, nil
		}
		if !errors.IsNotFound(err) {
			return nil, err
		}

		// 2. Fallback: try the apiresourceschema virtual workspace.
		logger.V(2).Info("APIResourceSchema not found in workspace, trying VW fallback", "schema", name)

		clusterID, err := extractClusterID(clusterConfig)
		if err != nil {
			return nil, fmt.Errorf("cannot build VW fallback client: %w", err)
		}

		vwClient, err := r.newVWClient(clusterID)
		if err != nil {
			return nil, fmt.Errorf("failed to create VW client for cluster %q: %w", clusterID, err)
		}

		var vwSchema apisv1alpha1.APIResourceSchema
		if err := vwClient.Get(ctx, client.ObjectKey{Name: name}, &vwSchema); err != nil {
			return nil, fmt.Errorf("APIResourceSchema %q not found in workspace or VW: %w", name, err)
		}

		return &vwSchema, nil
	}
}

// getTemplateMapper returns a mapper that enqueues the owning APIBinding when
// an APIServiceExportTemplate changes.
func getTemplateMapper(clusterName string, cl cluster.Cluster) handler.TypedEventHandler[client.Object, mcreconcile.Request] {
	return handler.TypedEnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []mcreconcile.Request {
		annotations := obj.GetAnnotations()
		if annotations == nil {
			return nil
		}
		ownerName, ok := annotations[annotationOwnerBinding]
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
