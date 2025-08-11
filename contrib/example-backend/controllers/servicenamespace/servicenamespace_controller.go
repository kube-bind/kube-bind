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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	kubernetesclient "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/kube-bind/kube-bind/pkg/indexers"
	kubebindv1alpha2 "github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2"
	bindclient "github.com/kube-bind/kube-bind/sdk/client/clientset/versioned"
)

const (
	controllerName = "kube-bind-example-backend-servicenamespace"
)

// APIServiceNamespaceReconciler reconciles a APIServiceNamespace object.
type APIServiceNamespaceReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	bindClient             bindclient.Interface
	kubeClient             kubernetesclient.Interface
	informerScope          kubebindv1alpha2.InformerScope
	clusterScopedIsolation kubebindv1alpha2.Isolation
	reconciler             reconciler
}

// NewAPIServiceNamespaceReconciler returns a new APIServiceNamespaceReconciler to reconcile APIServiceNamespaces.
func NewAPIServiceNamespaceReconciler(
	ctx context.Context,
	c client.Client,
	scheme *runtime.Scheme,
	config *rest.Config,
	cache cache.Cache,
	scope kubebindv1alpha2.InformerScope,
	isolation kubebindv1alpha2.Isolation,
) (*APIServiceNamespaceReconciler, error) {
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

	// Set up field indexers for APIServiceNamespaces
	if err := cache.IndexField(ctx, &kubebindv1alpha2.APIServiceNamespace{}, indexers.ServiceNamespaceByNamespace,
		indexers.IndexServiceNamespaceByNamespaceControllerRuntime); err != nil {
		return nil, fmt.Errorf("failed to setup ServiceNamespaceByNamespace indexer: %w", err)
	}

	r := &APIServiceNamespaceReconciler{
		Client:                 c,
		Scheme:                 scheme,
		bindClient:             bindClient,
		kubeClient:             kubeClient,
		informerScope:          scope,
		clusterScopedIsolation: isolation,
		reconciler: reconciler{
			scope: scope,

			getNamespace: func(name string) (*corev1.Namespace, error) {
				var ns corev1.Namespace
				key := types.NamespacedName{Name: name}
				if err := cache.Get(ctx, key, &ns); err != nil {
					return nil, err
				}
				return &ns, nil
			},
			createNamespace: func(ctx context.Context, ns *corev1.Namespace) (*corev1.Namespace, error) {
				return kubeClient.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
			},
			deleteNamespace: func(ctx context.Context, name string) error {
				return kubeClient.CoreV1().Namespaces().Delete(ctx, name, metav1.DeleteOptions{})
			},

			getRoleBinding: func(ns, name string) (*rbacv1.RoleBinding, error) {
				var rb rbacv1.RoleBinding
				key := types.NamespacedName{Namespace: ns, Name: name}
				if err := cache.Get(ctx, key, &rb); err != nil {
					return nil, err
				}
				return &rb, nil
			},
			createRoleBinding: func(ctx context.Context, crb *rbacv1.RoleBinding) (*rbacv1.RoleBinding, error) {
				return kubeClient.RbacV1().RoleBindings(crb.Namespace).Create(ctx, crb, metav1.CreateOptions{})
			},
			updateRoleBinding: func(ctx context.Context, crb *rbacv1.RoleBinding) (*rbacv1.RoleBinding, error) {
				return kubeClient.RbacV1().RoleBindings(crb.Namespace).Update(ctx, crb, metav1.UpdateOptions{})
			},
		},
	}

	return r, nil
}

// createNamespaceMapper creates a mapping function for Namespace changes.
func (r *APIServiceNamespaceReconciler) createNamespaceMapper() handler.MapFunc {
	return func(ctx context.Context, obj client.Object) []reconcile.Request {
		namespace := obj.(*corev1.Namespace)
		nsKey := namespace.Name

		var serviceNamespaces kubebindv1alpha2.APIServiceNamespaceList
		if err := r.List(ctx, &serviceNamespaces, client.MatchingFields{indexers.ServiceNamespaceByNamespace: nsKey}); err != nil {
			return []reconcile.Request{}
		}

		var result []reconcile.Request
		for _, sns := range serviceNamespaces.Items {
			result = append(result, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: sns.Namespace,
					Name:      sns.Name,
				},
			})
		}

		return result
	}
}

// createClusterBindingMapper creates a mapping function for ClusterBinding changes.
func (r *APIServiceNamespaceReconciler) createClusterBindingMapper() handler.MapFunc {
	return func(ctx context.Context, obj client.Object) []reconcile.Request {
		clusterBinding := obj.(*kubebindv1alpha2.ClusterBinding)
		ns := clusterBinding.Namespace

		var serviceNamespaces kubebindv1alpha2.APIServiceNamespaceList
		if err := r.List(ctx, &serviceNamespaces, client.InNamespace(ns)); err != nil {
			return []reconcile.Request{}
		}

		var result []reconcile.Request
		for _, sns := range serviceNamespaces.Items {
			result = append(result, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: sns.Namespace,
					Name:      sns.Name,
				},
			})
		}

		return result
	}
}

// createServiceExportMapper creates a mapping function for APIServiceExport changes.
func (r *APIServiceNamespaceReconciler) createServiceExportMapper() handler.MapFunc {
	return func(ctx context.Context, obj client.Object) []reconcile.Request {
		serviceExport := obj.(*kubebindv1alpha2.APIServiceExport)
		ns := serviceExport.Namespace

		var serviceNamespaces kubebindv1alpha2.APIServiceNamespaceList
		if err := r.List(ctx, &serviceNamespaces, client.InNamespace(ns)); err != nil {
			return []reconcile.Request{}
		}

		var result []reconcile.Request
		for _, sns := range serviceNamespaces.Items {
			result = append(result, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: sns.Namespace,
					Name:      sns.Name,
				},
			})
		}

		return result
	}
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
func (r *APIServiceNamespaceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling APIServiceNamespace", "namespace", req.Namespace, "name", req.Name)

	// Fetch the APIServiceNamespace instance
	apiServiceNamespace := &kubebindv1alpha2.APIServiceNamespace{}
	if err := r.Get(ctx, req.NamespacedName, apiServiceNamespace); err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Handle deletion logic here
			nsName := req.Namespace + "-" + req.Name
			if err := r.reconciler.deleteNamespace(ctx, nsName); err != nil && !errors.IsNotFound(err) {
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
	if err := r.reconciler.reconcile(ctx, apiServiceNamespace); err != nil {
		logger.Error(err, "Failed to reconcile APIServiceNamespace")
		return ctrl.Result{}, err
	}

	// Update status if it has changed
	if !reflect.DeepEqual(original.Status, apiServiceNamespace.Status) {
		err := r.Status().Update(ctx, apiServiceNamespace)
		if err != nil {
			logger.Error(err, "Failed to update APIServiceNamespace status")
			return ctrl.Result{}, fmt.Errorf("failed to update APIServiceNamespace status: %w", err)
		}
		logger.Info("APIServiceNamespace status updated", "namespace", apiServiceNamespace.Namespace, "name", apiServiceNamespace.Name)
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *APIServiceNamespaceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kubebindv1alpha2.APIServiceNamespace{}).
		Watches(
			&corev1.Namespace{},
			handler.EnqueueRequestsFromMapFunc(r.createNamespaceMapper()),
		).
		Watches(
			&kubebindv1alpha2.ClusterBinding{},
			handler.EnqueueRequestsFromMapFunc(r.createClusterBindingMapper()),
		).
		Watches(
			&kubebindv1alpha2.APIServiceExport{},
			handler.EnqueueRequestsFromMapFunc(r.createServiceExportMapper()),
		).
		Named(controllerName).
		Complete(r)
}
