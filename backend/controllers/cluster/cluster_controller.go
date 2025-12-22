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

package cluster

import (
	"context"
	"fmt"
	"reflect"

	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"
	mcbuilder "sigs.k8s.io/multicluster-runtime/pkg/builder"
	mcmanager "sigs.k8s.io/multicluster-runtime/pkg/manager"
	mcreconcile "sigs.k8s.io/multicluster-runtime/pkg/reconcile"

	kubebindv1alpha2 "github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2"
)

const (
	controllerName = "kube-bind-backend-cluster"
)

// ClusterReconciler reconciles a Cluster object.
type ClusterReconciler struct {
	manager    mcmanager.Manager
	opts       controller.TypedOptions[mcreconcile.Request]
	reconciler reconciler
}

// NewClusterReconciler returns a new ClusterReconciler to reconcile Clusters
// ands its resources.
func NewClusterReconciler(
	_ context.Context,
	mgr mcmanager.Manager,
	opts controller.TypedOptions[mcreconcile.Request],
	allowedGroups []string,
	allowedUsers []string,
) (*ClusterReconciler, error) {
	r := &ClusterReconciler{
		manager: mgr,
		opts:    opts,
		reconciler: reconciler{
			allowedGroups: allowedGroups,
			allowedUsers:  allowedUsers,
		},
	}

	return r, nil
}

//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterrolebindings,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *ClusterReconciler) Reconcile(ctx context.Context, req mcreconcile.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling Cluster", "request", req)

	cl, err := r.manager.GetCluster(ctx, req.ClusterName)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get client for cluster %q: %w", req.ClusterName, err)
	}

	client := cl.GetClient()
	cache := cl.GetCache()

	cluster := &kubebindv1alpha2.Cluster{}
	if err := client.Get(ctx, req.NamespacedName, cluster); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Cluster not found, skipping reconciliation")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("failed to get Cluster: %w", err)
	}

	original := cluster.DeepCopy()
	if err := r.reconciler.reconcile(ctx, client, cache, cluster); err != nil {
		logger.Error(err, "Failed to reconcile Cluster")
		return ctrl.Result{}, err
	}

	if !reflect.DeepEqual(original, cluster) {
		err := client.Update(ctx, cluster)
		if err != nil {
			logger.Error(err, "Failed to update Cluster status")
			return ctrl.Result{}, fmt.Errorf("failed to update Cluster status: %w", err)
		}
		logger.Info("Cluster status updated")
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ClusterReconciler) SetupWithManager(mgr mcmanager.Manager) error {
	return mcbuilder.ControllerManagedBy(mgr).
		For(&kubebindv1alpha2.Cluster{}).
		Owns(&rbacv1.ClusterRole{}).
		Owns(&rbacv1.ClusterRoleBinding{}).
		Owns(&rbacv1.RoleBinding{}).
		WithOptions(r.opts).
		Named(controllerName).
		Complete(r)
}
