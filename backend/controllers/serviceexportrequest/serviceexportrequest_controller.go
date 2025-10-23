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
	"reflect"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	controllerName = "kube-bind-backend-serviceexportrequest"
)

// APIServiceExportRequestReconciler reconciles a APIServiceExportRequest object.
type APIServiceExportRequestReconciler struct {
	manager mcmanager.Manager
	opts    controller.TypedOptions[mcreconcile.Request]

	informerScope          kubebindv1alpha2.InformerScope
	clusterScopedIsolation kubebindv1alpha2.Isolation
	reconciler             reconciler
}

// NewAPIServiceExportRequestReconciler returns a new APIServiceExportRequestReconciler to reconcile APIServiceExportRequests.
func NewAPIServiceExportRequestReconciler(
	ctx context.Context,
	mgr mcmanager.Manager,
	opts controller.TypedOptions[mcreconcile.Request],
	scope kubebindv1alpha2.InformerScope,
	isolation kubebindv1alpha2.Isolation,
	schemaSource string,
) (*APIServiceExportRequestReconciler, error) {
	// Set up field indexers for APIServiceExportRequests
	if err := mgr.GetFieldIndexer().IndexField(ctx, &kubebindv1alpha2.APIServiceExportRequest{}, indexers.ServiceExportRequestByServiceExport,
		indexers.IndexServiceExportRequestByServiceExport); err != nil {
		return nil, fmt.Errorf("failed to setup ServiceExportRequestByServiceExport indexer: %w", err)
	}

	if err := mgr.GetFieldIndexer().IndexField(ctx, &kubebindv1alpha2.APIServiceExportRequest{}, indexers.ServiceExportRequestByGroupResource,
		indexers.IndexServiceExportRequestByGroupResource); err != nil {
		return nil, fmt.Errorf("failed to setup ServiceExportRequestByGroupResource indexer: %w", err)
	}

	r := &APIServiceExportRequestReconciler{
		manager:                mgr,
		opts:                   opts,
		informerScope:          scope,
		clusterScopedIsolation: isolation,
		reconciler: reconciler{
			informerScope:          scope,
			clusterScopedIsolation: isolation,
			schemaSource:           schemaSource,
			getBoundSchema: func(ctx context.Context, cl client.Client, namespace, name string) (*kubebindv1alpha2.BoundSchema, error) {
				var schema kubebindv1alpha2.BoundSchema
				key := types.NamespacedName{Namespace: namespace, Name: name}
				if err := cl.Get(ctx, key, &schema); err != nil {
					return nil, err
				}
				return &schema, nil
			},
			getServiceExport: func(ctx context.Context, cache cache.Cache, ns, name string) (*kubebindv1alpha2.APIServiceExport, error) {
				var export kubebindv1alpha2.APIServiceExport
				key := types.NamespacedName{Namespace: ns, Name: name}
				if err := cache.Get(ctx, key, &export); err != nil {
					return nil, err
				}
				return &export, nil
			},
			createServiceExport: func(ctx context.Context, cl client.Client, resource *kubebindv1alpha2.APIServiceExport) error {
				return cl.Create(ctx, resource)
			},
			createBoundSchema: func(ctx context.Context, cl client.Client, schema *kubebindv1alpha2.BoundSchema) error {
				return cl.Create(ctx, schema)
			},
			deleteServiceExportRequest: func(ctx context.Context, cl client.Client, ns, name string) error {
				return cl.Delete(ctx, &kubebindv1alpha2.APIServiceExportRequest{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: ns,
						Name:      name,
					},
				})
			},
		},
	}

	return r, nil
}

// getServiceExportRequestMapper creates a mapping function for ServiceExport changes.
func getServiceExportRequestMapper(clusterName string, cl cluster.Cluster) handler.TypedEventHandler[client.Object, mcreconcile.Request] {
	return handler.TypedEnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []mcreconcile.Request {
		serviceExport := obj.(*kubebindv1alpha2.APIServiceExport)
		seKey := serviceExport.Namespace + "/" + serviceExport.Name

		c := cl.GetClient()

		var requests kubebindv1alpha2.APIServiceExportRequestList
		if err := c.List(ctx, &requests, client.MatchingFields{indexers.ServiceExportRequestByServiceExport: seKey}); err != nil {
			return []mcreconcile.Request{}
		}

		var result []mcreconcile.Request
		for _, req := range requests.Items {
			result = append(result, mcreconcile.Request{
				Request: reconcile.Request{
					NamespacedName: types.NamespacedName{
						Namespace: req.Namespace,
						Name:      req.Name,
					},
				},
				ClusterName: clusterName,
			})
		}

		return result
	})
}

// getBoundSchemaMapper creates a mapping function for BoundSchema changes.
func getBoundSchemaMapper(clusterName string, cl cluster.Cluster) handler.TypedEventHandler[client.Object, mcreconcile.Request] {
	return handler.TypedEnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []mcreconcile.Request {
		boundSchema := obj.(*kubebindv1alpha2.BoundSchema)
		boundSchemaKey := boundSchema.Spec.Names.Plural + "." + boundSchema.Spec.Group
		c := cl.GetClient()

		var requests kubebindv1alpha2.APIServiceExportRequestList
		if err := c.List(ctx, &requests, client.MatchingFields{indexers.ServiceExportRequestByGroupResource: boundSchemaKey}); err != nil {
			return []mcreconcile.Request{}
		}

		var result []mcreconcile.Request
		for _, req := range requests.Items {
			result = append(result, mcreconcile.Request{
				Request: reconcile.Request{
					NamespacedName: types.NamespacedName{
						Namespace: req.Namespace,
						Name:      req.Name,
					},
				},
				ClusterName: clusterName,
			})
		}
		return result
	})
}

//+kubebuilder:rbac:groups=kube-bind.io,resources=apiserviceexportrequests,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kube-bind.io,resources=apiserviceexportrequests/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kube-bind.io,resources=apiserviceexportrequests/finalizers,verbs=update
//+kubebuilder:rbac:groups=kube-bind.io,resources=apiserviceexports,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kube-bind.io,resources=apiresourceschemas,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kube-bind.io,resources=apiservicenamespaces,verbs=get;list;watch;create

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *APIServiceExportRequestReconciler) Reconcile(ctx context.Context, req mcreconcile.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling APIServiceExportRequest", "cluster", req.ClusterName, "namespace", req.Namespace, "name", req.Name)

	cl, err := r.manager.GetCluster(ctx, req.ClusterName)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get client for cluster %q: %w", req.ClusterName, err)
	}

	client := cl.GetClient()
	cache := cl.GetCache()

	// Fetch the APIServiceExportRequest instance
	apiServiceExportRequest := &kubebindv1alpha2.APIServiceExportRequest{}
	if err := client.Get(ctx, req.NamespacedName, apiServiceExportRequest); err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			logger.Info("APIServiceExportRequest not found, ignoring")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{}, fmt.Errorf("failed to get APIServiceExportRequest: %w", err)
	}

	// Create a copy to modify
	original := apiServiceExportRequest.DeepCopy()

	// Run the reconciliation logic
	if err := r.reconciler.reconcile(ctx, client, cache, apiServiceExportRequest); err != nil {
		logger.Error(err, "Failed to reconcile APIServiceExportRequest")
		if !reflect.DeepEqual(original.Status.Phase, apiServiceExportRequest.Status.Phase) {
			if err := client.Status().Update(ctx, apiServiceExportRequest); err != nil {
				logger.Error(err, "Failed to update APIServiceExportRequest status")
				return ctrl.Result{}, fmt.Errorf("failed to update APIServiceExportRequest status: %w", err)
			}
			logger.Info("APIServiceExportRequest status updated", "namespace", apiServiceExportRequest.Namespace, "name", apiServiceExportRequest.Name)
		}

		return ctrl.Result{}, err
	}

	// Update status if it has changed
	if !reflect.DeepEqual(original.Status, apiServiceExportRequest.Status) {
		if err := client.Status().Update(ctx, apiServiceExportRequest); err != nil {
			logger.Error(err, "Failed to update APIServiceExportRequest status")
			return ctrl.Result{}, fmt.Errorf("failed to update APIServiceExportRequest status: %w", err)
		}
		logger.Info("APIServiceExportRequest status updated", "namespace", apiServiceExportRequest.Namespace, "name", apiServiceExportRequest.Name)
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *APIServiceExportRequestReconciler) SetupWithManager(mgr mcmanager.Manager) error {
	return mcbuilder.ControllerManagedBy(mgr).
		For(&kubebindv1alpha2.APIServiceExportRequest{}).
		Watches(
			&kubebindv1alpha2.APIServiceExport{},
			getServiceExportRequestMapper,
		).
		Watches(
			&kubebindv1alpha2.BoundSchema{},
			getBoundSchemaMapper,
		).
		WithOptions(r.opts).
		Named(controllerName).
		Complete(r)
}
