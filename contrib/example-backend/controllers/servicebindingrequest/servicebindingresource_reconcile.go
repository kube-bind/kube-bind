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

package servicebindingrequest

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"

	kubebindv1alpha1 "github.com/kube-bind/kube-bind/pkg/apis/kubebind/v1alpha1"
	conditionsapi "github.com/kube-bind/kube-bind/pkg/apis/third_party/conditions/apis/conditions/v1alpha1"
	"github.com/kube-bind/kube-bind/pkg/apis/third_party/conditions/util/conditions"
)

type reconciler struct {
	deleteServiceBindingRequest func(ctx context.Context, ns, name string) error

	getServiceExport    func(ns, name string) (*kubebindv1alpha1.APIServiceExport, error)
	createServiceExport func(ctx context.Context, resource *kubebindv1alpha1.APIServiceExport) (*kubebindv1alpha1.APIServiceExport, error)
}

func (r *reconciler) reconcile(ctx context.Context, req *kubebindv1alpha1.APIServiceBindingRequest) error {
	var errs []error

	if err := r.ensureExports(ctx, req); err != nil {
		errs = append(errs, err)
	}

	conditions.SetSummary(req)

	return utilerrors.NewAggregate(errs)
}

func (r *reconciler) ensureExports(ctx context.Context, req *kubebindv1alpha1.APIServiceBindingRequest) error {
	logger := klog.FromContext(ctx)
	var errs []error

	if req.Status.Export == "" {
		// create new export
		export := &kubebindv1alpha1.APIServiceExport{
			ObjectMeta: metav1.ObjectMeta{
				Name: req.Name,
			},
			Spec: kubebindv1alpha1.APIServiceExportSpec{
				Scope: kubebindv1alpha1.ClusterScope, // TODO: implement namespace scope
			},
		}
		for _, res := range req.Spec.Resources {
			export.Spec.Resources = append(export.Spec.Resources, kubebindv1alpha1.APIServiceExportGroupResource{
				GroupResource: res.GroupResource,
			})
		}
		logger.V(2).Info("Creating service export %s/%s", export.Namespace, export.Name)
		var err error
		if _, err = r.createServiceExport(ctx, export); err != nil && !apierrors.IsAlreadyExists(err) {
			export.GenerateName = req.Name + "-"
			export.Name = ""

			logger.V(2).Info("Creation of service export %s/%s failed. Trying with a generated name", export.Namespace, export.Name)
			if _, err = r.createServiceExport(ctx, export); err != nil {
				errs = append(errs, err)
				return utilerrors.NewAggregate(errs)
			}
		}
		if err != nil {
			errs = append(errs, err)
			return utilerrors.NewAggregate(errs)
		}

		req.Status.Export = export.Name // waiting for export to be ready for phase update
		return nil
	}

	export, err := r.getServiceExport(req.Namespace, req.Status.Export)
	if err != nil && !apierrors.IsNotFound(err) {
		errs = append(errs, err)
		return utilerrors.NewAggregate(errs)
	} else if apierrors.IsNotFound(err) {
		conditions.MarkFalse(
			export,
			kubebindv1alpha1.APIServiceBindingRequestConditionExportsReady,
			"ServiceExportNotFound",
			conditionsapi.ConditionSeverityError,
			"APIServiceExport %s in the service provider cluster not found",
			req.Status.Export,
		)
		return nil
	}

	if c := conditions.Get(export, conditionsapi.ReadyCondition); c == nil {
		conditions.MarkFalse(
			export,
			kubebindv1alpha1.APIServiceBindingRequestConditionExportsReady,
			"Unknown",
			conditionsapi.ConditionSeverityInfo,
			"APIServiceExport %s in the service provider cluster has no Ready condition",
			req.Status.Export,
		)
		return nil
	} else if c.Status != corev1.ConditionTrue {
		conditions.MarkFalse(
			export,
			kubebindv1alpha1.APIServiceBindingRequestConditionExportsReady,
			c.Reason,
			c.Severity,
			c.Message,
			req.Status.Export,
		)
		return nil
	}
	conditions.MarkTrue(export, kubebindv1alpha1.APIServiceBindingRequestConditionExportsReady)

	if req.Status.Phase == kubebindv1alpha1.APIServiceBindingRequestPhasePending {
		if time.Since(req.CreationTimestamp.Time) > time.Minute {
			req.Status.Phase = kubebindv1alpha1.APIServiceBindingRequestPhaseFailed
		}
		req.Status.Phase = kubebindv1alpha1.APIServiceBindingRequestPhaseSucceeded
	} else if time.Since(req.CreationTimestamp.Time) > 10*time.Minute {
		logger.Info("Deleting service binding request %s/%s", req.Namespace, req.Name, "reason", "timeout", "age", time.Since(req.CreationTimestamp.Time))
		return r.deleteServiceBindingRequest(ctx, req.Namespace, req.Status.Export)
	}

	return utilerrors.NewAggregate(errs)
}
