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

package clusterbinding

import (
	"context"
	"fmt"
	"reflect"

	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
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

	kubebindv1alpha2 "github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2"
)

const (
	controllerName = "kube-bind-backend-clusterbinding"
)

// ClusterBindingReconciler reconciles a ClusterBinding object.
type ClusterBindingReconciler struct {
	manager    mcmanager.Manager
	opts       controller.TypedOptions[mcreconcile.Request]
	scope      kubebindv1alpha2.InformerScope
	reconciler reconciler
}

// NewClusterBindingReconciler returns a new ClusterBindingReconciler to reconcile ClusterBindings.
func NewClusterBindingReconciler(
	_ context.Context,
	mgr mcmanager.Manager,
	opts controller.TypedOptions[mcreconcile.Request],
	scope kubebindv1alpha2.InformerScope,
) (*ClusterBindingReconciler, error) {
	r := &ClusterBindingReconciler{
		manager: mgr,
		opts:    opts,
		scope:   scope,
		reconciler: reconciler{
			scope: scope,
			listServiceExports: func(ctx context.Context, cache cache.Cache, ns string) ([]*kubebindv1alpha2.APIServiceExport, error) {
				var list kubebindv1alpha2.APIServiceExportList
				if err := cache.List(ctx, &list, client.InNamespace(ns)); err != nil {
					return nil, err
				}
				var exports []*kubebindv1alpha2.APIServiceExport
				for i := range list.Items {
					exports = append(exports, &list.Items[i])
				}
				return exports, nil
			},
			getBoundSchema: func(ctx context.Context, cache cache.Cache, namespace, name string) (*kubebindv1alpha2.BoundSchema, error) {
				result := &kubebindv1alpha2.BoundSchema{}
				err := cache.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, result)
				if err != nil {
					return nil, fmt.Errorf("failed to get BoundSchema %q: %w", name, err)
				}
				return result, nil
			},
			getClusterRole: func(ctx context.Context, cache cache.Cache, name string) (*rbacv1.ClusterRole, error) {
				var role rbacv1.ClusterRole
				key := types.NamespacedName{Name: name}
				if err := cache.Get(ctx, key, &role); err != nil {
					return nil, err
				}
				return &role, nil
			},
			createClusterRole: func(ctx context.Context, client client.Client, binding *rbacv1.ClusterRole) error {
				return client.Create(ctx, binding)
			},
			updateClusterRole: func(ctx context.Context, client client.Client, binding *rbacv1.ClusterRole) error {
				return client.Update(ctx, binding)
			},
			getClusterRoleBinding: func(ctx context.Context, cache cache.Cache, name string) (*rbacv1.ClusterRoleBinding, error) {
				var binding rbacv1.ClusterRoleBinding
				key := types.NamespacedName{Name: name}
				if err := cache.Get(ctx, key, &binding); err != nil {
					return nil, err
				}
				return &binding, nil
			},
			createClusterRoleBinding: func(ctx context.Context, client client.Client, binding *rbacv1.ClusterRoleBinding) error {
				return client.Create(ctx, binding)
			},
			updateClusterRoleBinding: func(ctx context.Context, client client.Client, binding *rbacv1.ClusterRoleBinding) error {
				return client.Update(ctx, binding)
			},
			deleteClusterRoleBinding: func(ctx context.Context, client client.Client, name string) error {
				binding := &rbacv1.ClusterRoleBinding{
					ObjectMeta: metav1.ObjectMeta{Name: name},
				}
				return client.Delete(ctx, binding)
			},
			getNamespace: func(ctx context.Context, cache cache.Cache, name string) (*v1.Namespace, error) {
				var ns v1.Namespace
				key := types.NamespacedName{Name: name}
				if err := cache.Get(ctx, key, &ns); err != nil {
					return nil, err
				}
				return &ns, nil
			},
			createRoleBinding: func(ctx context.Context, client client.Client, binding *rbacv1.RoleBinding) error {
				return client.Create(ctx, binding)
			},
			updateRoleBinding: func(ctx context.Context, client client.Client, binding *rbacv1.RoleBinding) error {
				return client.Update(ctx, binding)
			},
			getRoleBinding: func(ctx context.Context, cache cache.Cache, ns, name string) (*rbacv1.RoleBinding, error) {
				var binding rbacv1.RoleBinding
				key := types.NamespacedName{Namespace: ns, Name: name}
				if err := cache.Get(ctx, key, &binding); err != nil {
					return nil, err
				}
				return &binding, nil
			},
		},
	}

	return r, nil
}

//+kubebuilder:rbac:groups=kubebind.k8s.io,resources=clusterbindings,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kubebind.k8s.io,resources=clusterbindings/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kubebind.k8s.io,resources=clusterbindings/finalizers,verbs=update
//+kubebuilder:rbac:groups=kubebind.k8s.io,resources=apiserviceexports,verbs=get;list;watch
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterrolebindings,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *ClusterBindingReconciler) Reconcile(ctx context.Context, req mcreconcile.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling ClusterBinding", "request", req)

	cl, err := r.manager.GetCluster(ctx, req.ClusterName)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get client for cluster %q: %w", req.ClusterName, err)
	}

	client := cl.GetClient()
	cache := cl.GetCache()

	// Fetch the ClusterBinding instance
	clusterBinding := &kubebindv1alpha2.ClusterBinding{}
	if err := client.Get(ctx, req.NamespacedName, clusterBinding); err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			logger.Info("ClusterBinding not found, ignoring")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{}, fmt.Errorf("failed to get ClusterBinding: %w", err)
	}

	// Create a copy to modify
	original := clusterBinding.DeepCopy()

	// Run the reconciliation logic
	if err := r.reconciler.reconcile(ctx, client, cache, clusterBinding); err != nil {
		logger.Error(err, "Failed to reconcile ClusterBinding")
		return ctrl.Result{}, err
	}

	// Update status if it has changed
	if !reflect.DeepEqual(original, clusterBinding) {
		err := client.Update(ctx, clusterBinding)
		if err != nil {
			logger.Error(err, "Failed to update ClusterBinding status")
			return ctrl.Result{}, fmt.Errorf("failed to update ClusterBinding status: %w", err)
		}
		logger.Info("ClusterBinding status updated")
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ClusterBindingReconciler) SetupWithManager(mgr mcmanager.Manager) error {
	return mcbuilder.ControllerManagedBy(mgr).
		For(&kubebindv1alpha2.ClusterBinding{}).
		Owns(&rbacv1.ClusterRole{}).
		Owns(&rbacv1.ClusterRoleBinding{}).
		Owns(&rbacv1.RoleBinding{}).
		Watches(
			&kubebindv1alpha2.APIServiceExport{},
			mapAPIResourceSchema,
		).
		WithOptions(r.opts).
		Named(controllerName).
		Complete(r)
}

func mapAPIResourceSchema(clusterName string, cl cluster.Cluster) handler.TypedEventHandler[client.Object, mcreconcile.Request] {
	return handler.TypedEnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []mcreconcile.Request {
		serviceExport, ok := obj.(*kubebindv1alpha2.APIServiceExport)
		if !ok {
			return nil
		}
		return []mcreconcile.Request{
			{
				Request: reconcile.Request{
					NamespacedName: client.ObjectKeyFromObject(serviceExport),
				},
				ClusterName: clusterName,
			},
		}
	})
}
