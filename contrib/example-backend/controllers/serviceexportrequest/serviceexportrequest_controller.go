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
	kubernetesclient "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	mcbuilder "sigs.k8s.io/multicluster-runtime/pkg/builder"
	mcmanager "sigs.k8s.io/multicluster-runtime/pkg/manager"
	mcreconcile "sigs.k8s.io/multicluster-runtime/pkg/reconcile"

	"github.com/kube-bind/kube-bind/pkg/indexers"
	kubebindv1alpha2 "github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2"
	bindclient "github.com/kube-bind/kube-bind/sdk/client/clientset/versioned"
)

const (
	controllerName = "kube-bind-example-backend-serviceexportrequest"
)

// APIServiceExportRequestReconciler reconciles a APIServiceExportRequest object.
type APIServiceExportRequestReconciler struct {
	manager mcmanager.Manager

	bindClient             bindclient.Interface
	kubeClient             kubernetesclient.Interface
	informerScope          kubebindv1alpha2.InformerScope
	clusterScopedIsolation kubebindv1alpha2.Isolation
	reconciler             reconciler
}

// NewAPIServiceExportRequestReconciler returns a new APIServiceExportRequestReconciler to reconcile APIServiceExportRequests.
func NewAPIServiceExportRequestReconciler(
	ctx context.Context,
	mgr mcmanager.Manager,
	config *rest.Config,
	scope kubebindv1alpha2.InformerScope,
	isolation kubebindv1alpha2.Isolation,
) (*APIServiceExportRequestReconciler, error) {
	config = rest.CopyConfig(config)
	config = rest.AddUserAgent(config, controllerName)

	bindClient, err := bindclient.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	kubeClient, err := kubernetesclient.NewForConfig(config)
	if err != nil {
		return nil, err
	}

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
		bindClient:             bindClient,
		kubeClient:             kubeClient,
		informerScope:          scope,
		clusterScopedIsolation: isolation,
		reconciler: reconciler{
			informerScope:          scope,
			clusterScopedIsolation: isolation,
			getAPIResourceSchema: func(ctx context.Context, cache cache.Cache, name string) (*kubebindv1alpha2.APIResourceSchema, error) {
				var schema kubebindv1alpha2.APIResourceSchema
				key := types.NamespacedName{Name: name}
				if err := cache.Get(ctx, key, &schema); err != nil {
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
			createServiceExport: func(ctx context.Context, resource *kubebindv1alpha2.APIServiceExport) (*kubebindv1alpha2.APIServiceExport, error) {
				return bindClient.KubeBindV1alpha2().APIServiceExports(resource.Namespace).Create(ctx, resource, metav1.CreateOptions{})
			},
			createAPIResourceSchema: func(ctx context.Context, schema *kubebindv1alpha2.APIResourceSchema) (*kubebindv1alpha2.APIResourceSchema, error) {
				return bindClient.KubeBindV1alpha2().APIResourceSchemas().Create(ctx, schema, metav1.CreateOptions{})
			},
			deleteServiceExportRequest: func(ctx context.Context, ns, name string) error {
				return bindClient.KubeBindV1alpha2().APIServiceExportRequests(ns).Delete(ctx, name, metav1.DeleteOptions{})
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

// getAPIResourceSchemaMapper creates a mapping function for APIResourceSchema changes.
func getAPIResourceSchemaMapper(clusterName string, cl cluster.Cluster) handler.TypedEventHandler[client.Object, mcreconcile.Request] {
	return handler.TypedEnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []mcreconcile.Request {
		apiResourceSchema := obj.(*kubebindv1alpha2.APIResourceSchema)
		apiResourceSchemaKey := apiResourceSchema.Name
		c := cl.GetClient()

		var requests kubebindv1alpha2.APIServiceExportRequestList
		if err := c.List(ctx, &requests, client.MatchingFields{indexers.ServiceExportRequestByGroupResource: apiResourceSchemaKey}); err != nil {
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

//+kubebuilder:rbac:groups=kubebind.k8s.io,resources=apiserviceexportrequests,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kubebind.k8s.io,resources=apiserviceexportrequests/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kubebind.k8s.io,resources=apiserviceexportrequests/finalizers,verbs=update
//+kubebuilder:rbac:groups=kubebind.k8s.io,resources=apiserviceexports,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kubebind.k8s.io,resources=apiresourceschemas,verbs=get;list;watch;create;update;patch;delete

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
	if err := r.reconciler.reconcile(ctx, cache, apiServiceExportRequest); err != nil {
		logger.Error(err, "Failed to reconcile APIServiceExportRequest")
		return ctrl.Result{}, err
	}

	// Update status if it has changed
	if !reflect.DeepEqual(original.Status, apiServiceExportRequest.Status) {
		err := client.Status().Update(ctx, apiServiceExportRequest)
		if err != nil {
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
			&kubebindv1alpha2.APIResourceSchema{},
			getAPIResourceSchemaMapper,
		).
		Named(controllerName).
		Complete(r)
}
