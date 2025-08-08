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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kubebindv1alpha2 "github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2"
	bindclient "github.com/kube-bind/kube-bind/sdk/client/clientset/versioned"
)

const (
	controllerName = "kube-bind-example-backend-clusterbinding"
)

// ClusterBindingReconciler reconciles a ClusterBinding object.
type ClusterBindingReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	scope      kubebindv1alpha2.InformerScope
	bindClient bindclient.Interface
	reconciler reconciler
}

// NewClusterBindingReconciler returns a new ClusterBindingReconciler to reconcile ClusterBindings.
func NewClusterBindingReconciler(
	client client.Client,
	scheme *runtime.Scheme,
	config *rest.Config,
	scope kubebindv1alpha2.InformerScope,
	cache cache.Cache,
) (*ClusterBindingReconciler, error) {
	config = rest.CopyConfig(config)
	config = rest.AddUserAgent(config, controllerName)

	bindClient, err := bindclient.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	r := &ClusterBindingReconciler{
		Client:     client,
		Scheme:     scheme,
		scope:      scope,
		bindClient: bindClient,
		reconciler: reconciler{
			scope: scope,
			listServiceExports: func(ctx context.Context, ns string) ([]*kubebindv1alpha2.APIServiceExport, error) {
				var list kubebindv1alpha2.APIServiceExportList
				if err := cache.List(ctx, &list); err != nil {
					return nil, err
				}
				var exports []*kubebindv1alpha2.APIServiceExport
				for i := range list.Items {
					if list.Items[i].Namespace == ns || ns == "" {
						exports = append(exports, &list.Items[i])
					}
				}
				return exports, nil
			},
			getAPIResourceSchema: func(ctx context.Context, namespace, name string) (*kubebindv1alpha2.APIResourceSchema, error) {
				return bindClient.KubeBindV1alpha2().APIResourceSchemas().Get(ctx, name, metav1.GetOptions{})
			},
			getClusterRole: func(ctx context.Context, name string) (*rbacv1.ClusterRole, error) {
				var role rbacv1.ClusterRole
				key := types.NamespacedName{Name: name}
				if err := client.Get(ctx, key, &role); err != nil {
					return nil, err
				}
				return &role, nil
			},
			createClusterRole: func(ctx context.Context, binding *rbacv1.ClusterRole) (*rbacv1.ClusterRole, error) {
				if err := client.Create(ctx, binding); err != nil {
					return nil, err
				}
				return binding, nil
			},
			updateClusterRole: func(ctx context.Context, binding *rbacv1.ClusterRole) (*rbacv1.ClusterRole, error) {
				if err := client.Update(ctx, binding); err != nil {
					return nil, err
				}
				return binding, nil
			},
			getClusterRoleBinding: func(ctx context.Context, name string) (*rbacv1.ClusterRoleBinding, error) {
				var binding rbacv1.ClusterRoleBinding
				key := types.NamespacedName{Name: name}
				if err := cache.Get(ctx, key, &binding); err != nil {
					return nil, err
				}
				return &binding, nil
			},
			createClusterRoleBinding: func(ctx context.Context, binding *rbacv1.ClusterRoleBinding) (*rbacv1.ClusterRoleBinding, error) {
				if err := client.Create(ctx, binding); err != nil {
					return nil, err
				}
				return binding, nil
			},
			updateClusterRoleBinding: func(ctx context.Context, binding *rbacv1.ClusterRoleBinding) (*rbacv1.ClusterRoleBinding, error) {
				if err := client.Update(ctx, binding); err != nil {
					return nil, err
				}
				return binding, nil
			},
			deleteClusterRoleBinding: func(ctx context.Context, name string) error {
				binding := &rbacv1.ClusterRoleBinding{
					ObjectMeta: metav1.ObjectMeta{Name: name},
				}
				return client.Delete(ctx, binding)
			},
			getNamespace: func(ctx context.Context, name string) (*v1.Namespace, error) {
				var ns v1.Namespace
				key := types.NamespacedName{Name: name}
				if err := cache.Get(ctx, key, &ns); err != nil {
					return nil, err
				}
				return &ns, nil
			},
			createRoleBinding: func(ctx context.Context, ns string, binding *rbacv1.RoleBinding) (*rbacv1.RoleBinding, error) {
				binding.Namespace = ns
				if err := client.Create(ctx, binding); err != nil {
					return nil, err
				}
				return binding, nil
			},
			updateRoleBinding: func(ctx context.Context, ns string, binding *rbacv1.RoleBinding) (*rbacv1.RoleBinding, error) {
				binding.Namespace = ns
				if err := client.Update(ctx, binding); err != nil {
					return nil, err
				}
				return binding, nil
			},
			getRoleBinding: func(ctx context.Context, ns, name string) (*rbacv1.RoleBinding, error) {
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

// mapServiceExportToClusterBinding maps APIServiceExport events to ClusterBinding reconcile requests.
func mapServiceExportToClusterBinding(ctx context.Context, obj client.Object) []reconcile.Request {
	// Extract namespace from the APIServiceExport
	serviceExport := obj.(*kubebindv1alpha2.APIServiceExport)

	// The original logic created a ClusterBinding key as "namespace/cluster"
	return []reconcile.Request{
		{
			NamespacedName: types.NamespacedName{
				Namespace: serviceExport.Namespace,
				Name:      "cluster", // This matches the original logic: ns + "/cluster"
			},
		},
	}
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
func (r *ClusterBindingReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling ClusterBinding", "namespace", req.Namespace, "name", req.Name)

	// Fetch the ClusterBinding instance
	clusterBinding := &kubebindv1alpha2.ClusterBinding{}
	if err := r.Get(ctx, req.NamespacedName, clusterBinding); err != nil {
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
	if err := r.reconciler.reconcile(ctx, clusterBinding); err != nil {
		logger.Error(err, "Failed to reconcile ClusterBinding")
		return ctrl.Result{}, err
	}

	// Update status if it has changed
	if !reflect.DeepEqual(original, clusterBinding) {
		err := r.Update(ctx, clusterBinding)
		if err != nil {
			logger.Error(err, "Failed to update ClusterBinding status")
			return ctrl.Result{}, fmt.Errorf("failed to update ClusterBinding status: %w", err)
		}
		logger.Info("ClusterBinding status updated", "namespace", clusterBinding.Namespace, "name", clusterBinding.Name)
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ClusterBindingReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kubebindv1alpha2.ClusterBinding{}).
		Owns(&rbacv1.ClusterRole{}).
		Owns(&rbacv1.ClusterRoleBinding{}).
		Owns(&rbacv1.RoleBinding{}).
		Watches(
			&kubebindv1alpha2.APIServiceExport{},
			handler.EnqueueRequestsFromMapFunc(mapServiceExportToClusterBinding),
		).
		Named(controllerName).
		Complete(r)
}
