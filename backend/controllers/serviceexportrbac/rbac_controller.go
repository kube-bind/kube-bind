/*
Copyright 2025 The Kube Bind Authors.

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

package serviceexportrbac

import (
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
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

	kubebindv1alpha2 "github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2"
)

const (
	controllerName = "kube-bind-backend-serviceexport-rbac"
)

type APIServiceExportRBACReconciler struct {
	manager    mcmanager.Manager
	opts       controller.TypedOptions[mcreconcile.Request]
	reconciler reconciler
}

func NewAPIServiceExportRBACReconciler(
	ctx context.Context,
	mgr mcmanager.Manager,
	scope kubebindv1alpha2.InformerScope,
	opts controller.TypedOptions[mcreconcile.Request],
) (*APIServiceExportRBACReconciler, error) {
	return &APIServiceExportRBACReconciler{
		manager: mgr,
		opts:    opts,

		reconciler: reconciler{
			scope: scope,
			// Namespace related.
			listServiceNamespaces: func(ctx context.Context, cache cache.Cache, namespace string) ([]*kubebindv1alpha2.APIServiceNamespace, error) {
				var list kubebindv1alpha2.APIServiceNamespaceList
				if err := cache.List(ctx, &list, client.InNamespace(namespace)); err != nil {
					return nil, err
				}
				var serviceNamespaces []*kubebindv1alpha2.APIServiceNamespace
				for i := range list.Items {
					serviceNamespaces = append(serviceNamespaces, &list.Items[i])
				}
				return serviceNamespaces, nil
			},
			getNamespace: func(ctx context.Context, cache cache.Cache, name string) (*v1.Namespace, error) {
				var ns v1.Namespace
				key := types.NamespacedName{Name: name}
				if err := cache.Get(ctx, key, &ns); err != nil {
					return nil, err
				}
				return &ns, nil
			},
			// ClusterRole.
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
			// ClusterRoleBinding.
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
			// Role.
			getRole: func(ctx context.Context, cache cache.Cache, namespace, name string) (*rbacv1.Role, error) {
				var role rbacv1.Role
				key := types.NamespacedName{Namespace: namespace, Name: name}
				if err := cache.Get(ctx, key, &role); err != nil {
					return nil, err
				}
				return &role, nil
			},
			createRole: func(ctx context.Context, client client.Client, role *rbacv1.Role) error {
				return client.Create(ctx, role)
			},
			updateRole: func(ctx context.Context, client client.Client, role *rbacv1.Role) error {
				return client.Update(ctx, role)
			},
			// RoleBinding.
			getRoleBinding: func(ctx context.Context, cache cache.Cache, ns, name string) (*rbacv1.RoleBinding, error) {
				var binding rbacv1.RoleBinding
				key := types.NamespacedName{Namespace: ns, Name: name}
				if err := cache.Get(ctx, key, &binding); err != nil {
					return nil, err
				}
				return &binding, nil
			},
			createRoleBinding: func(ctx context.Context, client client.Client, binding *rbacv1.RoleBinding) error {
				return client.Create(ctx, binding)
			},
			updateRoleBinding: func(ctx context.Context, client client.Client, binding *rbacv1.RoleBinding) error {
				return client.Update(ctx, binding)
			},
		},
	}, nil
}

func (r *APIServiceExportRBACReconciler) Reconcile(ctx context.Context, req mcreconcile.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling APIServiceExport", "request", req)

	cl, err := r.manager.GetCluster(ctx, req.ClusterName)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get client for cluster %q: %w", req.ClusterName, err)
	}

	client := cl.GetClient()
	cache := cl.GetCache()

	// Fetch the APIServiceExport instance
	export := kubebindv1alpha2.APIServiceExport{}
	if err := client.Get(ctx, req.NamespacedName, &export); err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			logger.Info("APIServiceExport not found, ignoring")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{}, fmt.Errorf("failed to get APIServiceExport: %w", err)
	}

	// Run the reconciliation logic
	if err := r.reconciler.reconcile(ctx, client, cache, &export); err != nil {
		logger.Error(err, "Failed to reconcile APIServiceExport")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *APIServiceExportRBACReconciler) SetupWithManager(mgr mcmanager.Manager) error {
	return mcbuilder.ControllerManagedBy(mgr).
		For(&kubebindv1alpha2.APIServiceExport{}).
		Watches(
			&kubebindv1alpha2.APIServiceNamespace{},
			mapAPIServiceNamespace,
		).
		WithOptions(r.opts).
		Named(controllerName).
		Complete(r)
}

func mapAPIServiceNamespace(clusterName string, cl cluster.Cluster) handler.TypedEventHandler[client.Object, mcreconcile.Request] {
	return handler.TypedEnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []mcreconcile.Request {
		serviceNamespace, ok := obj.(*kubebindv1alpha2.APIServiceNamespace)
		if !ok {
			return nil
		}

		c := cl.GetClient()

		var exports kubebindv1alpha2.APIServiceExportList
		if err := c.List(ctx, &exports, client.InNamespace(serviceNamespace.Namespace)); err != nil {
			return []mcreconcile.Request{}
		}

		var results []mcreconcile.Request
		for _, export := range exports.Items {
			results = append(results, mcreconcile.Request{
				Request: reconcile.Request{
					NamespacedName: types.NamespacedName{
						Namespace: export.Namespace,
						Name:      export.Name,
					},
				},
				ClusterName: clusterName,
			})
		}

		return results
	})
}
