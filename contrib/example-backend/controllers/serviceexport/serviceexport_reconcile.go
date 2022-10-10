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

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"

	kubebindv1alpha1 "github.com/kube-bind/kube-bind/pkg/apis/kubebind/v1alpha1"
	kubebindhelpers "github.com/kube-bind/kube-bind/pkg/apis/kubebind/v1alpha1/helpers"
	conditionsapi "github.com/kube-bind/kube-bind/pkg/apis/third_party/conditions/apis/conditions/v1alpha1"
	"github.com/kube-bind/kube-bind/pkg/apis/third_party/conditions/util/conditions"
)

type reconciler struct {
	getCRD                      func(name string) (*apiextensionsv1.CustomResourceDefinition, error)
	getServiceExportResource    func(ns, name string) (*kubebindv1alpha1.ServiceExportResource, error)
	createServiceExportResource func(ctx context.Context, resource *kubebindv1alpha1.ServiceExportResource) (*kubebindv1alpha1.ServiceExportResource, error)
	updateServiceExportResource func(ctx context.Context, resource *kubebindv1alpha1.ServiceExportResource) (*kubebindv1alpha1.ServiceExportResource, error)
	deleteServiceExportResource func(ctx context.Context, ns, name string) error
}

func (r *reconciler) reconcile(ctx context.Context, export *kubebindv1alpha1.ServiceExport) error {
	logger := klog.FromContext(ctx)
	var errs []error

	resourceInSync := true
	for _, gr := range export.Spec.Resources {
		name := gr.Resource + "." + gr.Group

		ser, err := r.getServiceExportResource(export.Namespace, name)
		if err != nil && !errors.IsNotFound(err) {
			return err
		}
		crd, err := r.getCRD(name)
		if err != nil && !errors.IsNotFound(err) {
			return err
		}

		if crd == nil {
			if ser != nil {
				// CRD missing => delete SER too
				logger.V(1).Info("Deleting ServiceExportResource because CRD is missing")
				if err := r.deleteServiceExportResource(ctx, export.Namespace, name); err != nil && !errors.IsNotFound(err) {
					return err
				}
			}

			if resourceInSync {
				conditions.MarkFalse(
					export,
					kubebindv1alpha1.ServiceExportConditionResourcesInSync,
					"CustomResourceDefinitionMissing",
					conditionsapi.ConditionSeverityError,
					"Referenced CustomResourceDefinition %s does not exist",
					name,
				)
				resourceInSync = false
			}
			continue
		}

		resource, err := kubebindhelpers.CRDToServiceExportResource(crd)
		if err != nil {
			if resourceInSync {
				conditions.MarkFalse(
					export,
					kubebindv1alpha1.ServiceExportConditionResourcesInSync,
					"CustomResourceDefinitionUpdateFailed",
					conditionsapi.ConditionSeverityError,
					"CustomResourceDefinition %s cannot be converted into a ServiceExportResource: %s",
					name, err,
				)
				resourceInSync = false
			}
			continue
		}
		resource.Namespace = export.Namespace

		if ser == nil {
			// ServiceExportResource missing
			logger.V(1).Info("Creating ServiceExportResource")
			if _, err := r.createServiceExportResource(ctx, resource); err != nil {
				errs = append(errs, err)
				continue
			}
		} else {
			// both exist, update ServiceExportResource
			logger.V(1).Info("Updating ServiceExportResource")
			resource.ObjectMeta = ser.ObjectMeta
			if _, err := r.updateServiceExportResource(ctx, resource); err != nil {
				errs = append(errs, err)
				continue
			}
		}
	}

	if resourceInSync {
		conditions.MarkTrue(export, kubebindv1alpha1.ServiceExportConditionResourcesInSync)
	}

	return utilerrors.NewAggregate(errs)
}
