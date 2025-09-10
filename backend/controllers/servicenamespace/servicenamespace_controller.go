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

package servicenamespace

import (
	"context"
	"fmt"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
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
	controllerName = "kube-bind-backend-servicenamespace"
)

// APIServiceNamespaceReconciler reconciles a APIServiceNamespace object.
type APIServiceNamespaceReconciler struct {
	manager mcmanager.Manager
	opts    controller.TypedOptions[mcreconcile.Request]

	informerScope          kubebindv1alpha2.InformerScope
	clusterScopedIsolation kubebindv1alpha2.Isolation
	reconciler             reconciler
}

// NewAPIServiceNamespaceReconciler returns a new APIServiceNamespaceReconciler to reconcile APIServiceNamespaces.
func NewAPIServiceNamespaceReconciler(
	ctx context.Context,
	mgr mcmanager.Manager,
	opts controller.TypedOptions[mcreconcile.Request],
	scope kubebindv1alpha2.InformerScope,
	isolation kubebindv1alpha2.Isolation,
) (*APIServiceNamespaceReconciler, error) {
	// Set up field indexers for APIServiceNamespaces
	if err := mgr.GetFieldIndexer().IndexField(ctx, &kubebindv1alpha2.APIServiceNamespace{}, indexers.ServiceNamespaceByNamespace,
		indexers.IndexServiceNamespaceByNamespaceControllerRuntime); err != nil {
		return nil, fmt.Errorf("failed to setup ServiceNamespaceByNamespace indexer: %w", err)
	}

	r := &APIServiceNamespaceReconciler{
		manager:                mgr,
		opts:                   opts,
		informerScope:          scope,
		clusterScopedIsolation: isolation,
		reconciler: reconciler{
			scope: scope,

			getNamespace: func(ctx context.Context, cache cache.Cache, name string) (*corev1.Namespace, error) {
				var ns corev1.Namespace
				key := types.NamespacedName{Name: name}
				if err := cache.Get(ctx, key, &ns); err != nil {
					return nil, err
				}
				return &ns, nil
			},
			createNamespace: func(ctx context.Context, client client.Client, ns *corev1.Namespace) error {
				return client.Create(ctx, ns)
			},
			deleteNamespace: func(ctx context.Context, client client.Client, name string) error {
				var ns corev1.Namespace
				key := types.NamespacedName{Name: name}
				if err := client.Get(ctx, key, &ns); err != nil {
					return err
				}
				return client.Delete(ctx, &ns)
			},

			getRoleBinding: func(ctx context.Context, cache cache.Cache, ns, name string) (*rbacv1.RoleBinding, error) {
				var rb rbacv1.RoleBinding
				key := types.NamespacedName{Namespace: ns, Name: name}
				if err := cache.Get(ctx, key, &rb); err != nil {
					return nil, err
				}
				return &rb, nil
			},
			createRoleBinding: func(ctx context.Context, client client.Client, crb *rbacv1.RoleBinding) error {
				return client.Create(ctx, crb)
			},
			updateRoleBinding: func(ctx context.Context, client client.Client, crb *rbacv1.RoleBinding) error {
				return client.Update(ctx, crb)
			},
		},
	}

	return r, nil
}

// getNamespaceMapper creates a mapping function for Namespace changes.
func getNamespaceMapper(clusterName string, cl cluster.Cluster) handler.TypedEventHandler[client.Object, mcreconcile.Request] {
	return handler.TypedEnqueueRequestsFromMapFunc[client.Object, mcreconcile.Request](func(ctx context.Context, obj client.Object) []mcreconcile.Request {
		namespace := obj.(*corev1.Namespace)
		nsKey := namespace.Name

		c := cl.GetClient()

		var serviceNamespaces kubebindv1alpha2.APIServiceNamespaceList
		if err := c.List(ctx, &serviceNamespaces, client.MatchingFields{indexers.ServiceNamespaceByNamespace: nsKey}); err != nil {
			return []mcreconcile.Request{}
		}

		var result []mcreconcile.Request
		for _, sns := range serviceNamespaces.Items {
			result = append(result, mcreconcile.Request{
				Request: reconcile.Request{
					NamespacedName: types.NamespacedName{
						Namespace: sns.Namespace,
						Name:      sns.Name,
					},
				},
				ClusterName: clusterName,
			})
		}
		return result
	})
}

// createClusterBindingMapper creates a mapping function for ClusterBinding changes.
func getClusterBindingMapper(clusterName string, cl cluster.Cluster) handler.TypedEventHandler[client.Object, mcreconcile.Request] {
	return handler.TypedEnqueueRequestsFromMapFunc[client.Object, mcreconcile.Request](func(ctx context.Context, obj client.Object) []mcreconcile.Request {
		clusterBinding := obj.(*kubebindv1alpha2.ClusterBinding)
		ns := clusterBinding.Namespace

		c := cl.GetClient()

		var serviceNamespaces kubebindv1alpha2.APIServiceNamespaceList
		if err := c.List(ctx, &serviceNamespaces, client.InNamespace(ns)); err != nil {
			return []mcreconcile.Request{}
		}

		var result []mcreconcile.Request
		for _, sns := range serviceNamespaces.Items {
			result = append(result, mcreconcile.Request{
				Request: reconcile.Request{
					NamespacedName: types.NamespacedName{
						Namespace: sns.Namespace,
						Name:      sns.Name,
					},
				},
				ClusterName: clusterName,
			})
		}

		return result
	})
}

// getServiceExportMapper creates a mapping function for APIServiceExport changes.
func getServiceExportMapper(clusterName string, cl cluster.Cluster) handler.TypedEventHandler[client.Object, mcreconcile.Request] {
	return handler.TypedEnqueueRequestsFromMapFunc[client.Object, mcreconcile.Request](func(ctx context.Context, obj client.Object) []mcreconcile.Request {
		serviceExport := obj.(*kubebindv1alpha2.APIServiceExport)
		ns := serviceExport.Namespace

		c := cl.GetClient()

		var serviceNamespaces kubebindv1alpha2.APIServiceNamespaceList
		if err := c.List(ctx, &serviceNamespaces, client.InNamespace(ns)); err != nil {
			return []mcreconcile.Request{}
		}

		var result []mcreconcile.Request
		for _, sns := range serviceNamespaces.Items {
			result = append(result, mcreconcile.Request{
				Request: reconcile.Request{
					NamespacedName: types.NamespacedName{
						Namespace: sns.Namespace,
						Name:      sns.Name,
					},
				},
				ClusterName: clusterName,
			})
		}

		return result
	})
}

//+kubebuilder:rbac:groups=kubebind.k8s.io,resources=apiservicenamespaces,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kubebind.k8s.io,resources=apiservicenamespaces/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kubebind.k8s.io,resources=apiservicenamespaces/finalizers,verbs=update
//+kubebuilder:rbac:groups=kubebind.k8s.io,resources=clusterbindings,verbs=get;list;watch
//+kubebuilder:rbac:groups=kubebind.k8s.io,resources=apiserviceexports,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *APIServiceNamespaceReconciler) Reconcile(ctx context.Context, req mcreconcile.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling APIServiceNamespace", "cluster", req.ClusterName, "namespace", req.Namespace, "name", req.Name)

	cl, err := r.manager.GetCluster(ctx, req.ClusterName)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get client for cluster %q: %w", req.ClusterName, err)
	}

	client := cl.GetClient()
	cache := cl.GetCache()

	// Fetch the APIServiceNamespace instance
	apiServiceNamespace := &kubebindv1alpha2.APIServiceNamespace{}
	if err := cache.Get(ctx, req.NamespacedName, apiServiceNamespace); err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Handle deletion logic here
			nsName := req.Namespace + "-" + req.Name
			if err := r.reconciler.deleteNamespace(ctx, client, nsName); err != nil && !errors.IsNotFound(err) {
				return ctrl.Result{}, fmt.Errorf("failed to delete namespace %q: %w", nsName, err)
			}
			logger.Info("APIServiceNamespace not found, ignoring")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{}, fmt.Errorf("failed to get APIServiceNamespace: %w", err)
	}

	// Create a copy to modify
	original := apiServiceNamespace.DeepCopy()

	// Run the reconciliation logic
	if err := r.reconciler.reconcile(ctx, client, cache, apiServiceNamespace); err != nil {
		logger.Error(err, "Failed to reconcile APIServiceNamespace")
		return ctrl.Result{}, err
	}

	// Update status if it has changed
	if !reflect.DeepEqual(original.Status, apiServiceNamespace.Status) {
		err := client.Status().Update(ctx, apiServiceNamespace)
		if err != nil {
			logger.Error(err, "Failed to update APIServiceNamespace status")
			return ctrl.Result{}, fmt.Errorf("failed to update APIServiceNamespace status: %w", err)
		}
		logger.Info("APIServiceNamespace status updated", "namespace", apiServiceNamespace.Namespace, "name", apiServiceNamespace.Name)
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *APIServiceNamespaceReconciler) SetupWithManager(mgr mcmanager.Manager) error {
	return mcbuilder.ControllerManagedBy(mgr).
		For(&kubebindv1alpha2.APIServiceNamespace{}).
		Watches(
			&corev1.Namespace{},
			getNamespaceMapper,
		).
		Watches(
			&kubebindv1alpha2.ClusterBinding{},
			getClusterBindingMapper,
		).
		Watches(
			&kubebindv1alpha2.APIServiceExport{},
			getServiceExportMapper,
		).
		Owns(&corev1.Namespace{}).
		Owns(&rbacv1.RoleBinding{}).
		Owns(&rbacv1.Role{}).
		WithOptions(r.opts).
		Named(controllerName).
		Complete(r)
}
