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

package serviceexportresource

import (
	"context"
	"strings"
	"sync"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"

	kubebindv1alpha1 "github.com/kube-bind/kube-bind/pkg/apis/kubebind/v1alpha1"
	conditionsapi "github.com/kube-bind/kube-bind/pkg/apis/third_party/conditions/apis/conditions/v1alpha1"
	"github.com/kube-bind/kube-bind/pkg/apis/third_party/conditions/util/conditions"
)

type reconciler struct {
	// consumerSecretRefKey is the namespace/name value of the ServiceBinding kubeconfig secret reference.
	consumerSecretRefKey string
	providerNamespace    string

	lock        sync.Mutex
	syncContext map[string]syncContext // by CRD name

	getCRD            func(name string) (*apiextensionsv1.CustomResourceDefinition, error)
	getServiceBinding func(name string) (*kubebindv1alpha1.ServiceBinding, error)
}

type syncContext struct {
	generation int64
	cancel     func()
}

func (r *reconciler) reconcile(ctx context.Context, name string, resource *kubebindv1alpha1.ServiceExportResource) error {
	logger := klog.FromContext(ctx)

	if resource == nil {
		// stop dangling syncers on delete
		r.lock.Lock()
		defer r.lock.Unlock()
		if c, found := r.syncContext[name]; found {
			logger.V(1).Info("Stopping ServiceExportResource sync", "reason", "ServiceExportResource deleted")
			c.cancel()
			delete(r.syncContext, resource.Name)
		}
		return nil
	}

	var errs []error
	crd, err := r.getCRD(resource.Name)
	if err != nil && !errors.IsNotFound(err) {
		return err
	} else if errors.IsNotFound(err) {
		// stop it
		r.lock.Lock()
		defer r.lock.Unlock()
		if c, found := r.syncContext[resource.Name]; found {
			logger.V(1).Info("Stopping ServiceExportResource sync", "reason", "NoCustomResourceDefinition")
			c.cancel()
			delete(r.syncContext, resource.Name)
		}

		conditions.MarkFalse(
			resource,
			kubebindv1alpha1.ServiceExportResourrceConditionSyncing,
			"CustomResourceDefinitionNotFound",
			conditionsapi.ConditionSeverityWarning,
			"No CustomResourceDefinition for this resource in the consumer cluster",
		)

		return nil
	}

	// any binding that references this CRD?
	foundBinding := false
	for _, ref := range crd.OwnerReferences {
		parts := strings.SplitN(ref.APIVersion, "/", 2)
		if parts[0] != kubebindv1alpha1.SchemeGroupVersion.Group || ref.Kind != "ServiceBinding" {
			continue
		}
		binding, err := r.getServiceBinding(ref.Name)
		if err != nil && !errors.IsNotFound(err) {
			return err
		} else if err != nil {
			continue
		}

		if binding.Spec.KubeconfigSecretRef.Namespace+"/"+binding.Spec.KubeconfigSecretRef.Name == r.consumerSecretRefKey {
			foundBinding = true
			break
		}
	}

	if !foundBinding {
		// stop it
		r.lock.Lock()
		defer r.lock.Unlock()
		if c, found := r.syncContext[resource.Name]; found {
			logger.V(1).Info("Stopping ServiceExportResource sync", "reason", "NoServiceBinding")
			c.cancel()
			delete(r.syncContext, resource.Name)
		}

		conditions.MarkFalse(
			resource,
			kubebindv1alpha1.ServiceExportResourrceConditionSyncing,
			"ServiceBindingNotFound",
			conditionsapi.ConditionSeverityWarning,
			"No ServiceBinding for this resource in the consumer cluster",
		)

		return nil
	}

	c, found := r.syncContext[resource.Name]
	if found {
		if c.generation == resource.Generation {
			conditions.MarkTrue(resource, kubebindv1alpha1.ServiceExportResourrceConditionSyncing)
			return nil // all as expected
		}

		// technically, we could be less aggressive here if nothing big changed in the resource. But ¯\_(ツ)_/¯

		r.lock.Lock()
		if c, found := r.syncContext[resource.Name]; found {
			logger.V(1).Info("Stopping ServiceExportResource sync", "reason", "GenerationChanged", "generation", resource.Generation)
			c.cancel()
			delete(r.syncContext, resource.Name)
		}
		r.lock.Unlock()
	}

	// start a new sync
	ctx, cancel := context.WithCancel(ctx)
	r.lock.Lock()
	defer r.lock.Unlock()
	r.syncContext[resource.Name] = syncContext{
		generation: resource.Generation,
		cancel:     cancel,
	}

	conditions.MarkTrue(resource, kubebindv1alpha1.ServiceExportResourrceConditionSyncing)

	return utilerrors.NewAggregate(errs)
}
