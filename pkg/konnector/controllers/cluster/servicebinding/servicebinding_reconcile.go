/*
Copyright 2022 The kube bind Authors.

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

package servicebinding

import (
	"context"

	"k8s.io/apimachinery/pkg/api/errors"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"

	kubebindv1alpha1 "github.com/kube-bind/kube-bind/pkg/apis/kubebind/v1alpha1"
	conditionsapi "github.com/kube-bind/kube-bind/pkg/apis/third_party/conditions/apis/conditions/v1alpha1"
	"github.com/kube-bind/kube-bind/pkg/apis/third_party/conditions/util/conditions"
)

type reconciler struct {
	providerNamespace string

	getServiceExport func(ns string) (*kubebindv1alpha1.ServiceExport, error)
}

func (r *reconciler) reconcile(ctx context.Context, binding *kubebindv1alpha1.ServiceBinding) error {
	var errs []error

	if err := r.ensureValidServiceExport(ctx, binding); err != nil {
		errs = append(errs, err)
	}

	conditions.SetSummary(binding)

	return utilerrors.NewAggregate(errs)
}

func (r *reconciler) ensureValidServiceExport(ctx context.Context, binding *kubebindv1alpha1.ServiceBinding) error {
	export, err := r.getServiceExport(binding.Spec.Export)
	if err != nil && !errors.IsNotFound(err) {
		return err
	} else if errors.IsNotFound(err) {
		conditions.MarkFalse(
			binding,
			kubebindv1alpha1.ServiceBindingConditionConnected,
			"ServiceExportNotFound",
			conditionsapi.ConditionSeverityError,
			"ServiceExport %s not found on the service provider cluster. Rerun kubectl bind for repair.",
			binding.Spec.Export,
		)
		return nil
	}

	if inSync := conditions.Get(export, kubebindv1alpha1.ServiceExportConditionSchemaInSync); inSync != nil {
		conditions.Set(binding, inSync)
	} else {
		conditions.MarkFalse(
			binding,
			kubebindv1alpha1.ServiceBindingConditionSchemaInSync,
			"Unknown",
			conditionsapi.ConditionSeverityInfo,
			"ServiceExport %s in the service provider cluster does not have a SchemaInSync condition. Rerun kubectl bind for repair or wait.",
			binding.Spec.Export,
		)
	}

	if ready := conditions.Get(export, conditionsapi.ReadyCondition); ready != nil {
		clone := *ready
		clone.Type = kubebindv1alpha1.ServiceExportConditionExportReady
		conditions.Set(binding, &clone)
	} else {
		conditions.MarkFalse(
			binding,
			kubebindv1alpha1.ServiceExportConditionExportReady,
			"Unknown",
			conditionsapi.ConditionSeverityInfo,
			"ServiceExport %s in the service provider cluster does not have a Ready condition.",
			binding.Spec.Export,
		)
	}

	conditions.MarkTrue(
		binding,
		kubebindv1alpha1.ServiceBindingConditionConnected,
	)

	return nil
}
