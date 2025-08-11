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

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
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
	controllerName = "kube-bind-example-backend-serviceexportrequest"
)

// APIServiceExportRequestReconciler reconciles a APIServiceExportRequest object.
type APIServiceExportRequestReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	bindClient             bindclient.Interface
	kubeClient             kubernetesclient.Interface
	informerScope          kubebindv1alpha2.InformerScope
	clusterScopedIsolation kubebindv1alpha2.Isolation
	reconciler             reconciler
}

// NewAPIServiceExportRequestReconciler returns a new APIServiceExportRequestReconciler to reconcile APIServiceExportRequests.
func NewAPIServiceExportRequestReconciler(
	ctx context.Context,
	c client.Client,
	scheme *runtime.Scheme,
	config *rest.Config,
	cache cache.Cache,
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
	if err := cache.IndexField(ctx, &kubebindv1alpha2.APIServiceExportRequest{}, indexers.ServiceExportRequestByServiceExport,
		indexers.IndexServiceExportRequestByServiceExportControllerRuntime); err != nil {
		return nil, fmt.Errorf("failed to setup ServiceExportRequestByServiceExport indexer: %w", err)
	}

	if err := cache.IndexField(ctx, &kubebindv1alpha2.APIServiceExportRequest{}, indexers.ServiceExportRequestByGroupResource,
		indexers.IndexServiceExportRequestByGroupResourceControllerRuntime); err != nil {
		return nil, fmt.Errorf("failed to setup ServiceExportRequestByGroupResource indexer: %w", err)
	}

	r := &APIServiceExportRequestReconciler{
		Client:                 c,
		Scheme:                 scheme,
		bindClient:             bindClient,
		kubeClient:             kubeClient,
		informerScope:          scope,
		clusterScopedIsolation: isolation,
		reconciler: reconciler{
			informerScope:          scope,
			clusterScopedIsolation: isolation,
			getCRD: func(ctx context.Context, name string) (*apiextensionsv1.CustomResourceDefinition, error) {
				var crd apiextensionsv1.CustomResourceDefinition
				key := types.NamespacedName{Name: name}
				if err := cache.Get(ctx, key, &crd); err != nil {
					return nil, err
				}
				return &crd, nil
			},
			getAPIResourceSchema: func(ctx context.Context, name string) (*kubebindv1alpha2.APIResourceSchema, error) {
				var schema kubebindv1alpha2.APIResourceSchema
				key := types.NamespacedName{Name: name}
				if err := cache.Get(ctx, key, &schema); err != nil {
					return nil, err
				}
				return &schema, nil
			},
			getServiceExport: func(ctx context.Context, ns, name string) (*kubebindv1alpha2.APIServiceExport, error) {
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

// createServiceExportRequestMapper creates a mapping function for ServiceExport changes.
func (r *APIServiceExportRequestReconciler) createServiceExportRequestMapper() handler.MapFunc {
	return func(ctx context.Context, obj client.Object) []reconcile.Request {
		serviceExport := obj.(*kubebindv1alpha2.APIServiceExport)
		seKey := serviceExport.Namespace + "/" + serviceExport.Name

		var requests kubebindv1alpha2.APIServiceExportRequestList
		if err := r.List(ctx, &requests, client.MatchingFields{indexers.ServiceExportRequestByServiceExport: seKey}); err != nil {
			return []reconcile.Request{}
		}

		var result []reconcile.Request
		for _, req := range requests.Items {
			result = append(result, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: req.Namespace,
					Name:      req.Name,
				},
			})
		}

		return result
	}
}

// createCRDMapper creates a mapping function for CRD changes.
func (r *APIServiceExportRequestReconciler) createCRDMapper() handler.MapFunc {
	return func(ctx context.Context, obj client.Object) []reconcile.Request {
		crd := obj.(*apiextensionsv1.CustomResourceDefinition)
		crdKey := crd.Name // CRDs are cluster-scoped

		var requests kubebindv1alpha2.APIServiceExportRequestList
		if err := r.List(ctx, &requests, client.MatchingFields{indexers.ServiceExportRequestByGroupResource: crdKey}); err != nil {
			return []reconcile.Request{}
		}

		var result []reconcile.Request
		for _, req := range requests.Items {
			result = append(result, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: req.Namespace,
					Name:      req.Name,
				},
			})
		}

		return result
	}
}

//+kubebuilder:rbac:groups=kubebind.k8s.io,resources=apiserviceexportrequests,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kubebind.k8s.io,resources=apiserviceexportrequests/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kubebind.k8s.io,resources=apiserviceexportrequests/finalizers,verbs=update
//+kubebuilder:rbac:groups=kubebind.k8s.io,resources=apiserviceexports,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kubebind.k8s.io,resources=apiresourceschemas,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apiextensions.k8s.io,resources=customresourcedefinitions,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *APIServiceExportRequestReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling APIServiceExportRequest", "namespace", req.Namespace, "name", req.Name)

	// Fetch the APIServiceExportRequest instance
	apiServiceExportRequest := &kubebindv1alpha2.APIServiceExportRequest{}
	if err := r.Get(ctx, req.NamespacedName, apiServiceExportRequest); err != nil {
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
	if err := r.reconciler.reconcile(ctx, apiServiceExportRequest); err != nil {
		logger.Error(err, "Failed to reconcile APIServiceExportRequest")
		return ctrl.Result{}, err
	}

	// Update status if it has changed
	if !reflect.DeepEqual(original.Status, apiServiceExportRequest.Status) {
		err := r.Status().Update(ctx, apiServiceExportRequest)
		if err != nil {
			logger.Error(err, "Failed to update APIServiceExportRequest status")
			return ctrl.Result{}, fmt.Errorf("failed to update APIServiceExportRequest status: %w", err)
		}
		logger.Info("APIServiceExportRequest status updated", "namespace", apiServiceExportRequest.Namespace, "name", apiServiceExportRequest.Name)
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *APIServiceExportRequestReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kubebindv1alpha2.APIServiceExportRequest{}).
		Watches(
			&kubebindv1alpha2.APIServiceExport{},
			handler.EnqueueRequestsFromMapFunc(r.createServiceExportRequestMapper()),
		).
		Watches(
			&apiextensionsv1.CustomResourceDefinition{},
			handler.EnqueueRequestsFromMapFunc(r.createCRDMapper()),
		).
		Named(controllerName).
		Complete(r)
}
