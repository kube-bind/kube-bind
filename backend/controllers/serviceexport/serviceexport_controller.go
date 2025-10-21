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
	"fmt"

	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	mcbuilder "sigs.k8s.io/multicluster-runtime/pkg/builder"
	mcmanager "sigs.k8s.io/multicluster-runtime/pkg/manager"
	mcreconcile "sigs.k8s.io/multicluster-runtime/pkg/reconcile"

	"github.com/kube-bind/kube-bind/pkg/indexers"
	kubebindv1alpha2 "github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2"
)

const (
	controllerName = "kube-bind-backend-serviceexport"
)

// APIServiceExportReconciler reconciles a APIServiceExport object.
type APIServiceExportReconciler struct {
	manager mcmanager.Manager
	opts    controller.TypedOptions[mcreconcile.Request]

	reconciler reconciler
}

// NewAPIServiceExportReconciler returns a new APIServiceExportReconciler to reconcile APIServiceExports.
func NewAPIServiceExportReconciler(
	ctx context.Context,
	mgr mcmanager.Manager,
	scope kubebindv1alpha2.InformerScope,
	opts controller.TypedOptions[mcreconcile.Request],
) (*APIServiceExportReconciler, error) {
	if err := mgr.GetFieldIndexer().IndexField(ctx, &kubebindv1alpha2.APIServiceExport{}, indexers.ServiceExportByBoundSchema,
		indexers.IndexServiceExportByBoundSchemaControllerRuntime); err != nil {
		return nil, fmt.Errorf("failed to setup ServiceExportByBoundSchema indexer: %w", err)
	}

	r := &APIServiceExportReconciler{
		manager: mgr,
		opts:    opts,
		reconciler: reconciler{
			scope: scope,
			getBoundSchema: func(ctx context.Context, cache cache.Cache, namespace, name string) (*kubebindv1alpha2.BoundSchema, error) {
				var schema kubebindv1alpha2.BoundSchema
				key := types.NamespacedName{Namespace: namespace, Name: name}
				if err := cache.Get(ctx, key, &schema); err != nil {
					return nil, err
				}
				return &schema, nil
			},
		},
	}

	return r, nil
}

//+kubebuilder:rbac:groups=kubebind.k8s.io,resources=apiserviceexports,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kubebind.k8s.io,resources=apiserviceexports/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kubebind.k8s.io,resources=apiserviceexports/finalizers,verbs=update
//+kubebuilder:rbac:groups=kubebind.k8s.io,resources=boundschemas,verbs=get;list;watch
//+kubebuilder:rbac:groups=kubebind.k8s.io,resources=boundschemas/status,verbs=get;update;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *APIServiceExportReconciler) Reconcile(ctx context.Context, req mcreconcile.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling APIServiceExport", "request", req)

	cl, err := r.manager.GetCluster(ctx, req.ClusterName)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get client for cluster %q: %w", req.ClusterName, err)
	}

	client := cl.GetClient()
	cache := cl.GetCache()

	// Fetch the APIServiceExport instance
	apiServiceExport := &kubebindv1alpha2.APIServiceExport{}
	if err := client.Get(ctx, req.NamespacedName, apiServiceExport); err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			logger.Info("APIServiceExport not found, ignoring")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{}, fmt.Errorf("failed to get APIServiceExport: %w", err)
	}

	// Create a copy to modify
	original := apiServiceExport.DeepCopy()

	// Run the reconciliation logic
	if err := r.reconciler.reconcile(ctx, cache, apiServiceExport); err != nil {
		logger.Error(err, "Failed to reconcile APIServiceExport")
		return ctrl.Result{}, err
	}

	// Update annotations changed (hash), we need to propagate it and requeue for status changes.
	// This is why we compare annotations only as we don't expect any changes to spec.
	// Status changes are handled below.
	if !equality.Semantic.DeepEqual(original.Annotations, apiServiceExport.Annotations) {
		if err := client.Update(ctx, apiServiceExport); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to update APIServiceExport: %w", err)
		}
		logger.Info("APIServiceExport hash updated", "namespace", apiServiceExport.Namespace, "name", apiServiceExport.Name)
		return ctrl.Result{Requeue: true}, nil
	}
	if !equality.Semantic.DeepEqual(original.Status, apiServiceExport.Status) {
		if err := client.Status().Update(ctx, apiServiceExport); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to update APIServiceExport status: %w", err)
		}
		logger.Info("APIServiceExport status updated", "namespace", apiServiceExport.Namespace, "name", apiServiceExport.Name)
	}

	return ctrl.Result{}, nil
}

// getAPIResourceSchemaMapper returns a mapper function that uses the manager to find related APIServiceExports.
func getBoundSchemaMapper(clusterName string, cl cluster.Cluster) handler.TypedEventHandler[client.Object, mcreconcile.Request] {
	return handler.TypedEnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []mcreconcile.Request {
		boundSchema := obj.(*kubebindv1alpha2.BoundSchema)
		boundSchemaKey := boundSchema.Spec.Names.Plural + "." + boundSchema.Spec.Group
		c := cl.GetClient()

		var exports kubebindv1alpha2.APIServiceExportList
		if err := c.List(ctx, &exports, client.MatchingFields{indexers.ServiceExportByBoundSchema: boundSchemaKey}); err != nil {
			return []mcreconcile.Request{}
		}

		var requests []mcreconcile.Request
		for _, export := range exports.Items {
			requests = append(requests, mcreconcile.Request{
				Request: reconcile.Request{
					NamespacedName: client.ObjectKeyFromObject(&export),
				},
				ClusterName: clusterName,
			})
		}

		return requests
	})
}

// SetupWithManager sets up the controller with the Manager.
func (r *APIServiceExportReconciler) SetupWithManager(mgr mcmanager.Manager) error {
	return mcbuilder.ControllerManagedBy(mgr).
		For(&kubebindv1alpha2.APIServiceExport{}).
		Watches(
			&kubebindv1alpha2.BoundSchema{},
			getBoundSchemaMapper,
		).
		WithOptions(r.opts).
		Named(controllerName).
		Complete(r)
}
