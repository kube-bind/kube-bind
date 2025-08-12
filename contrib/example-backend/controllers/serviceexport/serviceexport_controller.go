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

package serviceexport

import (
	"context"
	"fmt"

<<<<<<< HEAD
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/equality"
=======
>>>>>>> bcd22d9 (Exchange CRDInformers with APIResourceSchemaInformers)
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

	"github.com/kube-bind/kube-bind/pkg/indexers"
	kubebindv1alpha2 "github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2"
	bindclient "github.com/kube-bind/kube-bind/sdk/client/clientset/versioned"
)

const (
	controllerName = "kube-bind-example-backend-serviceexport"
)

// APIServiceExportReconciler reconciles a APIServiceExport object.
type APIServiceExportReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	bindClient bindclient.Interface
	reconciler reconciler
}

// NewAPIServiceExportReconciler returns a new APIServiceExportReconciler to reconcile APIServiceExports.
func NewAPIServiceExportReconciler(
	ctx context.Context,
	c client.Client,
	scheme *runtime.Scheme,
	config *rest.Config,
<<<<<<< HEAD
	cache cache.Cache,
) (*APIServiceExportReconciler, error) {
=======
	serviceExportInformer bindinformers.APIServiceExportInformer,
	apiResourceSchemaInformer bindinformers.APIResourceSchemaInformer,
) (*Controller, error) {
	queue := workqueue.NewTypedRateLimitingQueueWithConfig(workqueue.DefaultTypedControllerRateLimiter[string](), workqueue.TypedRateLimitingQueueConfig[string]{Name: controllerName})

	logger := klog.Background().WithValues("controller", controllerName)

>>>>>>> bcd22d9 (Exchange CRDInformers with APIResourceSchemaInformers)
	config = rest.CopyConfig(config)
	config = rest.AddUserAgent(config, controllerName)

	bindClient, err := bindclient.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	// Set up field indexer for APIServiceExports by CustomResourceDefinition name.
	if err := cache.IndexField(ctx, &kubebindv1alpha2.APIServiceExport{}, indexers.ServiceExportByCustomResourceDefinition,
		indexers.IndexServiceExportByCustomResourceDefinitionControllerRuntime); err != nil {
		return nil, fmt.Errorf("failed to setup ServiceExportByCustomResourceDefinition indexer: %w", err)
	}

	r := &APIServiceExportReconciler{
		Client:     c,
		Scheme:     scheme,
		bindClient: bindClient,
<<<<<<< HEAD
		reconciler: reconciler{
			getCRD: func(ctx context.Context, name string) (*apiextensionsv1.CustomResourceDefinition, error) {
				var crd apiextensionsv1.CustomResourceDefinition
				key := types.NamespacedName{Name: name}
				if err := cache.Get(ctx, key, &crd); err != nil {
					return nil, err
				}
				return &crd, nil
			},
=======

		serviceExportLister:  serviceExportInformer.Lister(),
		serviceExportIndexer: serviceExportInformer.Informer().GetIndexer(),

		apiResourceSchemaLister:  apiResourceSchemaInformer.Lister(),
		apiResourceSchemaIndexer: apiResourceSchemaInformer.Informer().GetIndexer(),

		reconciler: reconciler{
>>>>>>> bcd22d9 (Exchange CRDInformers with APIResourceSchemaInformers)
			getAPIResourceSchema: func(ctx context.Context, name string) (*kubebindv1alpha2.APIResourceSchema, error) {
				return bindClient.KubeBindV1alpha2().APIResourceSchemas().Get(ctx, name, metav1.GetOptions{})
			},
			deleteServiceExport: func(ctx context.Context, ns, name string) error {
				return bindClient.KubeBindV1alpha2().APIServiceExports(ns).Delete(ctx, name, metav1.DeleteOptions{})
			},
		},
	}

<<<<<<< HEAD
	return r, nil
=======
	indexers.AddIfNotPresentOrDie(serviceExportInformer.Informer().GetIndexer(), cache.Indexers{
		indexers.ServiceExportByAPIResourceSchema: indexers.IndexServiceExportByAPIResourceSchema,
	})

	if _, err := serviceExportInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj any) {
			c.enqueueServiceExport(logger, obj)
		},
		UpdateFunc: func(old, newObj any) {
			c.enqueueServiceExport(logger, newObj)
		},
		DeleteFunc: func(obj any) {
			c.enqueueServiceExport(logger, obj)
		},
	}); err != nil {
		return nil, err
	}

	if _, err := apiResourceSchemaInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj any) {
			c.enqueueAPIResourceSchema(logger, obj)
		},
		UpdateFunc: func(old, newObj any) {
			c.enqueueAPIResourceSchema(logger, newObj)
		},
		DeleteFunc: func(obj any) {
			c.enqueueAPIResourceSchema(logger, obj)
		},
	}); err != nil {
		return nil, err
	}

	return c, nil
>>>>>>> bcd22d9 (Exchange CRDInformers with APIResourceSchemaInformers)
}

// createCRDMapper creates a mapping function that can access the client for looking up related APIServiceExports.
func (r *APIServiceExportReconciler) createCRDMapper() handler.MapFunc {
	return func(ctx context.Context, obj client.Object) []reconcile.Request {
		crd := obj.(*apiextensionsv1.CustomResourceDefinition)

		// Use the field indexer to find APIServiceExports related to this CRD
		// The indexer key should match the CRD name (namespace/name format for namespaced objects)
		crdKey := crd.Name // CRDs are cluster-scoped, so just the name

<<<<<<< HEAD
		var exports kubebindv1alpha2.APIServiceExportList
		if err := r.List(ctx, &exports, client.MatchingFields{indexers.ServiceExportByCustomResourceDefinition: crdKey}); err != nil {
			return []reconcile.Request{}
=======
	bindClient bindclient.Interface

	serviceExportLister  bindlisters.APIServiceExportLister
	serviceExportIndexer cache.Indexer

	apiResourceSchemaLister  bindlisters.APIResourceSchemaLister
	apiResourceSchemaIndexer cache.Indexer

	reconciler

	commit CommitFunc
}

func (c *Controller) enqueueServiceExport(logger klog.Logger, obj any) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		runtime.HandleError(err)
		return
	}

	logger.V(2).Info("queueing APIServiceExport", "key", key)
	c.queue.Add(key)
}

func (c *Controller) enqueueAPIResourceSchema(logger klog.Logger, obj any) {
	apiResourceSchemaKey, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		runtime.HandleError(err)
		return
	}

	exports, err := c.serviceExportIndexer.ByIndex(indexers.ServiceExportByAPIResourceSchema, apiResourceSchemaKey)
	if err != nil {
		runtime.HandleError(err)
		return
	}

	for _, obj := range exports {
		export, ok := obj.(*kubebindv1alpha2.APIServiceExport)
		if !ok {
			runtime.HandleError(fmt.Errorf("unexpected type %T", obj))
			return
>>>>>>> bcd22d9 (Exchange CRDInformers with APIResourceSchemaInformers)
		}

		var requests []reconcile.Request
		for _, export := range exports.Items {
			requests = append(requests, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: export.Namespace,
					Name:      export.Name,
				},
			})
		}

		return requests
	}
}

//+kubebuilder:rbac:groups=kubebind.k8s.io,resources=apiserviceexports,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kubebind.k8s.io,resources=apiserviceexports/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kubebind.k8s.io,resources=apiserviceexports/finalizers,verbs=update
//+kubebuilder:rbac:groups=apiextensions.k8s.io,resources=customresourcedefinitions,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *APIServiceExportReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling APIServiceExport", "namespace", req.Namespace, "name", req.Name)

	// Fetch the APIServiceExport instance
	apiServiceExport := &kubebindv1alpha2.APIServiceExport{}
	if err := r.Get(ctx, req.NamespacedName, apiServiceExport); err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			logger.Info("APIServiceExport not found, ignoring")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{}, fmt.Errorf("failed to get APIServiceExport: %w", err)
	}

	// Create a copy to modify
	original := apiServiceExport.DeepCopy()

	// Run the reconciliation logic
	if err := r.reconciler.reconcile(ctx, apiServiceExport); err != nil {
		logger.Error(err, "Failed to reconcile APIServiceExport")
		return ctrl.Result{}, err
	}

	// Update status if it has changed
	if !equality.Semantic.DeepEqual(original, apiServiceExport) {
		err := r.Status().Update(ctx, apiServiceExport)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to update APIServiceExport status: %w", err)
		}
<<<<<<< HEAD
		logger.Info("APIServiceExport status updated", "namespace", apiServiceExport.Namespace, "name", apiServiceExport.Name)
=======
		logger.V(2).Info("queueing APIServiceExport", "key", key, "reason", "APIResourceSchema", "APIResourceSchemaKey", apiResourceSchemaKey)
		c.queue.Add(key)
>>>>>>> bcd22d9 (Exchange CRDInformers with APIResourceSchemaInformers)
	}

	return ctrl.Result{}, nil
}

<<<<<<< HEAD
// SetupWithManager sets up the controller with the Manager.
func (r *APIServiceExportReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kubebindv1alpha2.APIServiceExport{}).
		Watches(
			&apiextensionsv1.CustomResourceDefinition{},
			handler.EnqueueRequestsFromMapFunc(r.createCRDMapper()),
		).
		Named(controllerName).
		Complete(r)
=======
// Start starts the controller, which stops when ctx.Done() is closed.
func (c *Controller) Start(ctx context.Context, numThreads int) {
	defer runtime.HandleCrash()
	defer c.queue.ShutDown()

	logger := klog.FromContext(ctx).WithValues("controller", controllerName)

	logger.Info("Starting controller")
	defer logger.Info("Shutting down controller")

	for range numThreads {
		go wait.UntilWithContext(ctx, c.startWorker, time.Second)
	}

	<-ctx.Done()
}

func (c *Controller) startWorker(ctx context.Context) {
	defer runtime.HandleCrash()

	for c.processNextWorkItem(ctx) {
	}
}

func (c *Controller) processNextWorkItem(ctx context.Context) bool {
	// Wait until there is a new item in the working queue
	key, quit := c.queue.Get()
	if quit {
		return false
	}

	logger := klog.FromContext(ctx).WithValues("key", key)
	ctx = klog.NewContext(ctx, logger)
	logger.V(2).Info("processing key")

	// No matter what, tell the queue we're done with this key, to unblock
	// other workers.
	defer c.queue.Done(key)

	if err := c.process(ctx, key); err != nil {
		runtime.HandleError(fmt.Errorf("%q controller failed to sync %q, err: %w", controllerName, key, err))
		c.queue.AddRateLimited(key)
		return true
	}
	c.queue.Forget(key)
	return true
}

func (c *Controller) process(ctx context.Context, key string) error {
	ns, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(err)
		return nil // we cannot do anything
	}

	obj, err := c.serviceExportLister.APIServiceExports(ns).Get(name)
	if err != nil && !errors.IsNotFound(err) {
		return err
	} else if errors.IsNotFound(err) {
		return nil // nothing we can do
	}

	old := obj
	obj = obj.DeepCopy()

	var errs []error
	if err := c.reconcile(ctx, obj); err != nil {
		errs = append(errs, err)
	}

	// Regardless of whether reconcile returned an error or not, always try to patch status if needed. Return the
	// reconciliation error at the end.

	// If the object being reconciled changed as a result, update it.
	oldResource := &Resource{ObjectMeta: old.ObjectMeta, Spec: &old.Spec, Status: &old.Status}
	newResource := &Resource{ObjectMeta: obj.ObjectMeta, Spec: &obj.Spec, Status: &obj.Status}
	if err := c.commit(ctx, oldResource, newResource); err != nil {
		errs = append(errs, err)
	}

	return utilerrors.NewAggregate(errs)
>>>>>>> bcd22d9 (Exchange CRDInformers with APIResourceSchemaInformers)
}
