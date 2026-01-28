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

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
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
	}

	return r, nil
}

//+kubebuilder:rbac:groups=kube-bind.io,resources=clusterbindings,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kube-bind.io,resources=clusterbindings/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kube-bind.io,resources=clusterbindings/finalizers,verbs=update
//+kubebuilder:rbac:groups=kube-bind.io,resources=apiserviceexports,verbs=get;list;watch
//+kubebuilder:rbac:groups=kube-bind.io,resources=collections,verbs=get;list;watch
//+kubebuilder:rbac:groups=kube-bind.io,resources=apiserviceexporttemplates,verbs=get;list;watch

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
					NamespacedName: types.NamespacedName{
						Namespace: serviceExport.GetNamespace(),
						Name:      "cluster",
					},
				},
				ClusterName: clusterName,
			},
		}
	})
}
