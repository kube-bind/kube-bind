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
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	runtimeschema "k8s.io/apimachinery/pkg/runtime/schema"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/runtime"
	dynamicclient "k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	kubebindv1alpha1 "github.com/kube-bind/kube-bind/pkg/apis/kubebind/v1alpha1"
	conditionsapi "github.com/kube-bind/kube-bind/pkg/apis/third_party/conditions/apis/conditions/v1alpha1"
	"github.com/kube-bind/kube-bind/pkg/apis/third_party/conditions/util/conditions"
	bindlisters "github.com/kube-bind/kube-bind/pkg/client/listers/kubebind/v1alpha1"
	"github.com/kube-bind/kube-bind/pkg/konnector/controllers/cluster/claimedresources"
	"github.com/kube-bind/kube-bind/pkg/konnector/controllers/cluster/serviceexport/multinsinformer"
	"github.com/kube-bind/kube-bind/pkg/konnector/controllers/cluster/serviceexport/spec"
	"github.com/kube-bind/kube-bind/pkg/konnector/controllers/cluster/serviceexport/status"
	"github.com/kube-bind/kube-bind/pkg/konnector/controllers/dynamic"
)

type reconciler struct {
	// consumerSecretRefKey is the namespace/name value of the APIServiceBinding kubeconfig secret reference.
	consumerSecretRefKey     string
	providerNamespace        string
	serviceNamespaceInformer dynamic.Informer[bindlisters.APIServiceNamespaceLister]

	consumerConfig, providerConfig *rest.Config

	lock        sync.Mutex
	syncContext map[string]syncContext // by CRD name

	getCRD            func(name string) (*apiextensionsv1.CustomResourceDefinition, error)
	getServiceBinding func(name string) (*kubebindv1alpha1.APIServiceBinding, error)
}

type syncContext struct {
	generation int64
	cancel     func()
}

func (r *reconciler) reconcile(ctx context.Context, name string, export *kubebindv1alpha1.APIServiceExport) error {
	errs := []error{}

	if err := r.ensureControllers(ctx, name, export); err != nil {
		errs = append(errs, err)
	}

	if export != nil {
		if err := r.ensureServiceBindingConditionCopied(ctx, export); err != nil {
			errs = append(errs, err)
		}
		if err := r.ensureCRDConditionsCopied(ctx, export); err != nil {
			errs = append(errs, err)
		}
	}

	return utilerrors.NewAggregate(errs)
}

func (r *reconciler) ensureControllers(ctx context.Context, name string, export *kubebindv1alpha1.APIServiceExport) error {
	logger := klog.FromContext(ctx)

	if export == nil {
		// stop dangling syncers on delete
		r.lock.Lock()
		defer r.lock.Unlock()
		if c, found := r.syncContext[name]; found {
			logger.V(1).Info("Stopping APIServiceExport sync", "reason", "APIServiceExport deleted")
			c.cancel()
			delete(r.syncContext, name)
		}
		return nil
	}

	var errs []error
	crd, err := r.getCRD(export.Name)
	if err != nil && !errors.IsNotFound(err) {
		return err
	} else if errors.IsNotFound(err) {
		// stop it
		r.lock.Lock()
		defer r.lock.Unlock()
		if c, found := r.syncContext[export.Name]; found {
			logger.V(1).Info("Stopping APIServiceExport sync", "reason", "NoCustomResourceDefinition")
			c.cancel()
			delete(r.syncContext, export.Name)
		}

		return nil
	}

	// any binding that references this resource?
	binding, err := r.getServiceBinding(export.Name)
	if err != nil && !errors.IsNotFound(err) {
		return err
	}
	if binding == nil {
		// stop it
		r.lock.Lock()
		defer r.lock.Unlock()
		if c, found := r.syncContext[export.Name]; found {
			logger.V(1).Info("Stopping APIServiceExport sync", "reason", "NoAPIServiceExport")
			c.cancel()
			delete(r.syncContext, export.Name)
		}

		return nil
	}

	r.lock.Lock()
	c, found := r.syncContext[export.Name]
	if found {
		if c.generation == export.Generation {
			r.lock.Unlock()
			return nil // all as expected
		}

		// technically, we could be less aggressive here if nothing big changed in the resource, e.g. just schemas. But ¯\_(ツ)_/¯

		logger.V(1).Info("Stopping APIServiceExport sync", "reason", "GenerationChanged", "generation", export.Generation)
		c.cancel()
		delete(r.syncContext, export.Name)
	}
	r.lock.Unlock()

	// start a new syncer

	var syncVersion string
	for _, v := range export.Spec.Versions {
		if v.Served {
			syncVersion = v.Name
			break
		}
	}
	gvr := runtimeschema.GroupVersionResource{Group: export.Spec.Group, Version: syncVersion, Resource: export.Spec.Names.Plural}

	dynamicConsumerClient := dynamicclient.NewForConfigOrDie(r.consumerConfig)
	dynamicProviderClient := dynamicclient.NewForConfigOrDie(r.providerConfig)
	consumerInf := dynamicinformer.NewDynamicSharedInformerFactory(dynamicConsumerClient, time.Minute*30)

	var providerInf multinsinformer.GetterInformer
	if crd.Spec.Scope == apiextensionsv1.ClusterScoped || export.Spec.InformerScope == kubebindv1alpha1.ClusterScope {
		factory := dynamicinformer.NewDynamicSharedInformerFactory(dynamicProviderClient, time.Minute*30)
		factory.ForResource(gvr).Lister() // wire the GVR up in the informer factory
		providerInf = multinsinformer.GetterInformerWrapper{
			GVR:      gvr,
			Delegate: factory,
		}
	} else {
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

	var claimControllers []func(context.Context, int)
	for _, claim := range binding.Spec.PermissionClaims {
		claim := claim

		if claim.State != kubebindv1alpha1.ClaimAccepted {
			logger.Info("skipping non accepted claim", "claim", claim)
			continue
		}

		if claim.Selector.Owner == kubebindv1alpha1.Consumer {
			// TODO implement upsync
			continue
		}

		claimGVR := runtimeschema.GroupVersionResource{
			Group:    claim.Group,
			Version:  claim.Version,
			Resource: claim.Resource,
		}

		var providerInf multinsinformer.GetterInformer
		if claim.Global {
			factory := dynamicinformer.NewDynamicSharedInformerFactory(dynamicProviderClient, time.Minute*30)
			factory.ForResource(claimGVR).Lister() // wire the GVR up in the informer factory
			providerInf = multinsinformer.GetterInformerWrapper{
				GVR:      claimGVR,
				Delegate: factory,
			}
		} else {
			providerInf, err = multinsinformer.NewDynamicMultiNamespaceInformer(
				claimGVR,
				r.providerNamespace,
				r.providerConfig,
				r.serviceNamespaceInformer,
			)
			if err != nil {
				logger.Info("aborting", "error", err)
				return err
			}
		}
		claimedCtrl, err := claimedresources.NewController(
			claimGVR,
			r.providerNamespace,
			r.consumerConfig,
			r.providerConfig,
			consumerInf.ForResource(claimGVR),
			providerInf,
			r.serviceNamespaceInformer,
		)

		if err != nil {
			runtime.HandleError(err)
			return nil //nothing we can do here
		}
		logger.Info("creating claim reconciler", "gvr", claimGVR)

		claimControllers = append(claimControllers, func(ctx context.Context, i int) {
			providerInf.Start(ctx)

			providerSynced := providerInf.WaitForCacheSync(ctx.Done())
			logger.V(2).Info("Synced informers", "provider", providerSynced)

			claimedCtrl.Start(ctx, i)
		})

	}

	ctx, cancel := context.WithCancel(ctx)

	consumerInf.Start(ctx.Done())
	providerInf.Start(ctx)

	go func() {
		// to not block the main thread
		consumerSynced := consumerInf.WaitForCacheSync(ctx.Done())
		logger.V(2).Info("Synced informers", "consumer", consumerSynced)

		providerSynced := providerInf.WaitForCacheSync(ctx.Done())
		logger.V(2).Info("Synced informers", "provider", providerSynced)

		go specCtrl.Start(ctx, 1)
		go statusCtrl.Start(ctx, 1)

		for _, f := range claimControllers {
			go f(ctx, 1)
		}
	}()

	r.lock.Lock()
	defer r.lock.Unlock()
	if c, found := r.syncContext[export.Name]; found {
		c.cancel()
	}
	r.syncContext[export.Name] = syncContext{
		generation: export.Generation,
		cancel:     cancel,
	}

	return utilerrors.NewAggregate(errs)
}

func (r *reconciler) ensureServiceBindingConditionCopied(ctx context.Context, export *kubebindv1alpha1.APIServiceExport) error {
	binding, err := r.getServiceBinding(export.Name)
	if err != nil && !errors.IsNotFound(err) {
		return err
	} else if errors.IsNotFound(err) {
		conditions.MarkFalse(
			export,
			kubebindv1alpha1.APIServiceExportConditionConnected,
			"APIServiceBindingNotFound",
			conditionsapi.ConditionSeverityInfo,
			"No APIServiceBinding exists.",
		)

		conditions.MarkFalse(
			export,
			kubebindv1alpha1.APIServiceExportConditionConsumerInSync,
			"NA",
			conditionsapi.ConditionSeverityInfo,
			"No APIServiceBinding exists.",
		)

		return nil
	}

	conditions.MarkTrue(export, kubebindv1alpha1.APIServiceExportConditionConnected)

	if inSync := conditions.Get(binding, kubebindv1alpha1.APIServiceBindingConditionSchemaInSync); inSync != nil {
		inSync := inSync.DeepCopy()
		inSync.Type = kubebindv1alpha1.APIServiceExportConditionConsumerInSync
		conditions.Set(export, inSync)
	} else {
		conditions.MarkFalse(
			export,
			kubebindv1alpha1.APIServiceExportConditionConsumerInSync,
			"Unknown",
			conditionsapi.ConditionSeverityInfo,
			"APIServiceBinding %s in the consumer cluster does not have a SchemaInSync condition.",
			binding.Name,
		)
	}

	return nil
}

func (r *reconciler) ensureCRDConditionsCopied(ctx context.Context, export *kubebindv1alpha1.APIServiceExport) error {
	crd, err := r.getCRD(export.Name)
	if err != nil && !errors.IsNotFound(err) {
		return err
	} else if errors.IsNotFound(err) {
		return nil //nothing to copy.
	}

	exportIndex := map[conditionsapi.ConditionType]int{}
	for i, c := range export.Status.Conditions {
		exportIndex[c.Type] = i
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
		if i, found := exportIndex[conditionsapi.ConditionType(c.Type)]; found {
			export.Status.Conditions[i] = copied
		} else {
			export.Status.Conditions = append(export.Status.Conditions, copied)
		}
	}
	conditions.SetSummary(export)

	export.Status.AcceptedNames = crd.Status.AcceptedNames
	export.Status.StoredVersions = crd.Status.StoredVersions

	return nil
}
