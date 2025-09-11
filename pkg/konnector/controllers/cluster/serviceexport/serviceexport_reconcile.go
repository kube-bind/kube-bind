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
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtimeschema "k8s.io/apimachinery/pkg/runtime/schema"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/runtime"
	dynamicclient "k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	"github.com/kube-bind/kube-bind/pkg/konnector/controllers/cluster/serviceexport/multinsinformer"
	"github.com/kube-bind/kube-bind/pkg/konnector/controllers/cluster/serviceexport/spec"
	"github.com/kube-bind/kube-bind/pkg/konnector/controllers/cluster/serviceexport/status"
	"github.com/kube-bind/kube-bind/pkg/konnector/controllers/dynamic"
	kubebindv1alpha2 "github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2"
	conditionsapi "github.com/kube-bind/kube-bind/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
	"github.com/kube-bind/kube-bind/sdk/apis/third_party/conditions/util/conditions"
	bindlisters "github.com/kube-bind/kube-bind/sdk/client/listers/kubebind/v1alpha2"
)

type reconciler struct {
	// consumerSecretRefKey is the namespace/name value of the APIServiceBinding kubeconfig secret reference.
	consumerSecretRefKey     string
	providerNamespace        string
	serviceNamespaceInformer dynamic.Informer[bindlisters.APIServiceNamespaceLister]

	consumerConfig, providerConfig *rest.Config

	lock        sync.Mutex
	syncContext map[string]syncContext // by CRD name

	getCRD               func(name string) (*apiextensionsv1.CustomResourceDefinition, error)
	getServiceBinding    func(name string) (*kubebindv1alpha2.APIServiceBinding, error)
	getRemoteBoundSchema func(ctx context.Context, name string) (*kubebindv1alpha2.BoundSchema, error)

	updateRemoteBoundSchema func(ctx context.Context, boundSchema *kubebindv1alpha2.BoundSchema) error
}

type syncContext struct {
	generation int64
	cancel     func()
}

func (r *reconciler) reconcile(ctx context.Context, name string, export *kubebindv1alpha2.APIServiceExport) error {
	errs := []error{}

	if err := r.ensureControllers(ctx, name, export); err != nil {
		errs = append(errs, err)
	}

	if export != nil {
		if err := r.ensureServiceBindingConditionCopied(ctx, export); err != nil {
			errs = append(errs, err)
		}
		if err := r.ensureCRDConditionsCopiedToBoundSchema(ctx, export); err != nil {
			errs = append(errs, err)
		}
	}

	return utilerrors.NewAggregate(errs)
}

func (r *reconciler) ensureControllers(ctx context.Context, name string, export *kubebindv1alpha2.APIServiceExport) error {
	logger := klog.FromContext(ctx)

	if export == nil {
		// stop dangling syncers on delete
		r.lock.Lock()
		defer r.lock.Unlock()

		// Clean up any controllers associated with this export
		for key, c := range r.syncContext {
			if strings.HasSuffix(key, "."+name) {
				logger.V(1).Info("Stopping APIServiceExport sync", "key", key, "reason", "APIServiceExport deleted")
				c.cancel()
				delete(r.syncContext, key)
			}
		}
		return nil
	}

	// any binding that references this resource?
	binding, err := r.getServiceBinding(export.Name)
	if err != nil && !errors.IsNotFound(err) {
		return err
	}
	if binding == nil {
		// Stop all controllers for this export
		r.lock.Lock()
		defer r.lock.Unlock()
		for key, c := range r.syncContext {
			if strings.HasSuffix(key, "."+export.Name) {
				logger.V(1).Info("Stopping APIServiceExport sync", "key", key, "reason", "NoAPIServiceBinding")
				c.cancel()
				delete(r.syncContext, key)
			}
		}
		return nil
	}

	// Process each resource referenced by the export
	var errs []error
	processedSchemas := make(map[string]bool)

	for _, res := range export.Spec.Resources {
		name := res.Resource + "." + res.Group
		// Fetch the APIResourceSchema
		schema, err := r.getRemoteBoundSchema(ctx, name)
		if err != nil {
			if errors.IsNotFound(err) {
				// Stop the controller for this schema if it exists
				r.lock.Lock()
				key := name + "." + export.Name
				if c, found := r.syncContext[key]; found {
					logger.V(1).Info("Stopping APIServiceExport resource sync", "key", key, "reason", "BoundSchema not found")
					c.cancel()
					delete(r.syncContext, key)
				}
				r.lock.Unlock()
				continue
			}
			errs = append(errs, err)
			continue
		}

		// Start/update controller for this schema
		if err := r.ensureControllerForSchema(ctx, export, schema); err != nil {
			errs = append(errs, err)
		}

		processedSchemas[name] = true
	}

	// Stop controllers for schemas that are no longer referenced
	r.lock.Lock()
	for key, c := range r.syncContext {
		parts := strings.Split(key, ".")
		if len(parts) == 2 && parts[1] == export.Name {
			schemaName := parts[0]
			if !processedSchemas[schemaName] {
				logger.V(1).Info("Stopping APIServiceExport resource sync", "key", key, "reason", "Schema no longer referenced")
				c.cancel()
				delete(r.syncContext, key)
			}
		}
	}
	r.lock.Unlock()

	return utilerrors.NewAggregate(errs)
}

func (r *reconciler) ensureControllerForSchema(ctx context.Context, export *kubebindv1alpha2.APIServiceExport, schema *kubebindv1alpha2.BoundSchema) error {
	logger := klog.FromContext(ctx)
	key := schema.Name + "." + export.Name

	r.lock.Lock()
	c, found := r.syncContext[key]
	if found {
		if c.generation == export.Generation {
			r.lock.Unlock()
			return nil // all as expected
		}

		logger.V(1).Info("Stopping APIServiceExport resource sync", "key", key, "reason", "GenerationChanged", "generation", schema.Generation)
		c.cancel()
		delete(r.syncContext, key)
	}
	r.lock.Unlock()

	// start a new syncer
	var syncVersion string
	for _, v := range schema.Spec.Versions {
		if v.Served {
			syncVersion = v.Name
			break
		}
	}
	if syncVersion == "" {
		return fmt.Errorf("no served version found for BoundSchema %s", schema.Name)
	}

	gvr := runtimeschema.GroupVersionResource{
		Group:    schema.Spec.Group,
		Version:  syncVersion,
		Resource: schema.Spec.Names.Plural,
	}

	dynamicConsumerClient := dynamicclient.NewForConfigOrDie(r.consumerConfig)
	dynamicProviderClient := dynamicclient.NewForConfigOrDie(r.providerConfig)

	providerNamespaceUID := ""
	if pns, err := dynamicProviderClient.Resource(runtimeschema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "namespaces",
	}).Get(ctx, r.providerNamespace, metav1.GetOptions{}); err != nil {
		return err
	} else {
		providerNamespaceUID = string(pns.GetUID())
	}

	consumerInf := dynamicinformer.NewDynamicSharedInformerFactory(dynamicConsumerClient, time.Minute*30)

	var providerInf multinsinformer.GetterInformer
	if schema.Spec.Scope == apiextensionsv1.ClusterScoped || schema.Spec.InformerScope == kubebindv1alpha2.ClusterScope {
		factory := dynamicinformer.NewDynamicSharedInformerFactory(dynamicProviderClient, time.Minute*30)
		factory.ForResource(gvr).Lister() // wire the GVR up in the informer factory
		providerInf = multinsinformer.GetterInformerWrapper{
			GVR:      gvr,
			Delegate: factory,
		}
	} else {
		var err error
		providerInf, err = multinsinformer.NewDynamicMultiNamespaceInformer(
			gvr,
			r.providerNamespace,
			r.providerConfig,
			r.serviceNamespaceInformer,
		)
		if err != nil {
			return err
		}
	}

	specCtrl, err := spec.NewController(
		gvr,
		r.providerNamespace,
		providerNamespaceUID,
		r.consumerConfig,
		r.providerConfig,
		consumerInf.ForResource(gvr),
		providerInf,
		r.serviceNamespaceInformer,
	)
	if err != nil {
		runtime.HandleError(err)
		return nil // nothing we can do here
	}

	statusCtrl, err := status.NewController(
		gvr,
		r.providerNamespace,
		providerNamespaceUID,
		r.consumerConfig,
		r.providerConfig,
		consumerInf.ForResource(gvr),
		providerInf,
		r.serviceNamespaceInformer,
	)
	if err != nil {
		runtime.HandleError(err)
		return nil // nothing we can do here
	}

	// Create a new context for this controller
	ctxWithCancel, cancel := context.WithCancel(ctx)

	consumerInf.Start(ctxWithCancel.Done())
	providerInf.Start(ctxWithCancel)

	go func() {
		// to not block the main thread
		consumerSynced := consumerInf.WaitForCacheSync(ctxWithCancel.Done())
		logger.V(2).Info("Synced informers", "key", key, "consumer", consumerSynced)

		providerSynced := providerInf.WaitForCacheSync(ctxWithCancel.Done())
		logger.V(2).Info("Synced informers", "key", key, "provider", providerSynced)

		go specCtrl.Start(ctxWithCancel, 1)
		go statusCtrl.Start(ctxWithCancel, 1)
	}()

	r.lock.Lock()
	defer r.lock.Unlock()
	if c, found := r.syncContext[key]; found {
		c.cancel()
	}
	r.syncContext[key] = syncContext{
		generation: schema.Generation,
		cancel:     cancel,
	}

	return nil
}

func (r *reconciler) ensureCRDConditionsCopiedToBoundSchema(ctx context.Context, export *kubebindv1alpha2.APIServiceExport) error {
	if export == nil {
		return nil
	}
	var errs []error
	allValid := true // assume all BoundAPIResourceSchemas are valid
	for _, res := range export.Spec.Resources {
		name := res.Resource + "." + res.Group
		boundSchema, err := r.getRemoteBoundSchema(ctx, name)
		if err != nil {
			if errors.IsNotFound(err) {
				continue
			}
			errs = append(errs, err)
			continue
		}

		crd, err := r.getCRD(boundSchema.Name)
		if err != nil {
			if errors.IsNotFound(err) {
				continue
			}
			errs = append(errs, err)
			continue
		}

		boundSchemaIndex := map[conditionsapi.ConditionType]int{}
		for i, c := range boundSchema.Status.Conditions {
			boundSchemaIndex[c.Type] = i
		}
		for _, c := range crd.Status.Conditions {
			if conditionsapi.ConditionType(c.Type) == conditionsapi.ReadyCondition {
				continue
			}

			severity := conditionsapi.ConditionSeverityError
			if c.Status == apiextensionsv1.ConditionTrue {
				severity = conditionsapi.ConditionSeverityNone
			}
			copied := conditionsapi.Condition{
				Type:               conditionsapi.ConditionType(c.Type),
				Status:             corev1.ConditionStatus(c.Status),
				Severity:           severity, // CRD conditions have no severity
				LastTransitionTime: c.LastTransitionTime,
				Reason:             c.Reason,
				Message:            c.Message,
			}

			// update or append
			if i, found := boundSchemaIndex[conditionsapi.ConditionType(c.Type)]; found {
				boundSchema.Status.Conditions[i] = copied
			} else {
				boundSchema.Status.Conditions = append(boundSchema.Status.Conditions, copied)
			}
		}
		conditions.SetSummary(boundSchema)

		boundSchema.Status.AcceptedNames = crd.Status.AcceptedNames
		boundSchema.Status.StoredVersions = crd.Status.StoredVersions

		if err := r.updateRemoteBoundSchema(ctx, boundSchema); err != nil {
			errs = append(errs, err)
			allValid = false // at least one BoundSchemas is not valid
		}
	}

	// Set APIServiceExport Ready condition based on all BoundSchemas
	if allValid {
		conditions.MarkTrue(
			export,
			kubebindv1alpha2.APIServiceExportConditionConnected,
		)
	} else {
		conditions.MarkFalse(
			export,
			kubebindv1alpha2.APIServiceExportConditionConnected,
			"BoundSchemasNotValid",
			conditionsapi.ConditionSeverityWarning,
			"One or more BoundSchemas are not valid",
		)
	}

	return utilerrors.NewAggregate(errs)
}

func (r *reconciler) ensureServiceBindingConditionCopied(_ context.Context, export *kubebindv1alpha2.APIServiceExport) error {
	binding, err := r.getServiceBinding(export.Name)
	if err != nil && !errors.IsNotFound(err) {
		return err
	} else if errors.IsNotFound(err) {
		conditions.MarkFalse(
			export,
			kubebindv1alpha2.APIServiceExportConditionConnected,
			"APIServiceBindingNotFound",
			conditionsapi.ConditionSeverityInfo,
			"No APIServiceBinding exists.",
		)

		conditions.MarkFalse(
			export,
			kubebindv1alpha2.APIServiceExportConditionConsumerInSync,
			"NA",
			conditionsapi.ConditionSeverityInfo,
			"No APIServiceBinding exists.",
		)

		return nil
	}

	conditions.MarkTrue(export, kubebindv1alpha2.APIServiceExportConditionConnected)

	if inSync := conditions.Get(binding, kubebindv1alpha2.APIServiceBindingConditionSchemaInSync); inSync != nil {
		inSync := inSync.DeepCopy()
		inSync.Type = kubebindv1alpha2.APIServiceExportConditionConsumerInSync
		conditions.Set(export, inSync)
	} else {
		conditions.MarkFalse(
			export,
			kubebindv1alpha2.APIServiceExportConditionConsumerInSync,
			"Unknown",
			conditionsapi.ConditionSeverityInfo,
			"APIServiceBinding %s in the consumer cluster does not have a SchemaInSync condition.",
			binding.Name,
		)
	}

	return nil
}
