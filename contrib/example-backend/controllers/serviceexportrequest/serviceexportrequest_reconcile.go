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
	"time"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"

	kubebindv1alpha2 "github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2"
	"github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2/helpers"
	conditionsapi "github.com/kube-bind/kube-bind/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
	"github.com/kube-bind/kube-bind/sdk/apis/third_party/conditions/util/conditions"
)

type reconciler struct {
	informerScope          kubebindv1alpha2.InformerScope
	clusterScopedIsolation kubebindv1alpha2.Isolation

	getCRD                     func(name string) (*apiextensionsv1.CustomResourceDefinition, error)
	getAPIResourceSchema       func(ctx context.Context, name string) (*kubebindv1alpha2.APIResourceSchema, error)
	getServiceExport           func(ns, name string) (*kubebindv1alpha2.APIServiceExport, error)
	createServiceExport        func(ctx context.Context, resource *kubebindv1alpha2.APIServiceExport) (*kubebindv1alpha2.APIServiceExport, error)
	createAPIResourceSchema    func(ctx context.Context, schema *kubebindv1alpha2.APIResourceSchema) (*kubebindv1alpha2.APIResourceSchema, error)
	deleteServiceExportRequest func(ctx context.Context, namespace, name string) error
}

func (r *reconciler) reconcile(ctx context.Context, req *kubebindv1alpha2.APIServiceExportRequest) error {
	var errs []error

	if err := r.ensureExports(ctx, req); err != nil {
		errs = append(errs, err)
	}

	conditions.SetSummary(req)

	return utilerrors.NewAggregate(errs)
}

func (r *reconciler) ensureExports(ctx context.Context, req *kubebindv1alpha2.APIServiceExportRequest) error {
	logger := klog.FromContext(ctx)

	if req.Status.Phase == kubebindv1alpha2.APIServiceExportRequestPhasePending {
		failure := false
		for _, res := range req.Spec.Resources {
			// backend is created using CRD's as backup. But this is not required.
			name := res.Resource + "." + res.Group

			apiResourceSchema, err := r.getAPIResourceSchema(ctx, name)
			switch {
			case apierrors.IsNotFound(err):
				logger.V(1).Info("APIResourceSchema not found, continuing with fallback to CRD conversion to APIResourceSchema", "name", name)
				crd, err := r.getCRD(name)
				if err != nil && !apierrors.IsNotFound(err) {
					return err
				}
				if apierrors.IsNotFound(err) {
					conditions.MarkFalse(
						req,
						kubebindv1alpha2.APIServiceExportRequestConditionExportsReady,
						"CRDNotFound",
						conditionsapi.ConditionSeverityError,
						"CustomResourceDefinition %s in the service provider cluster not found",
						name,
					)
					failure = true
					break
				}
				schema, err := helpers.CRDToAPIResourceSchema(crd, "")
				if err != nil {
					return err
				}
				schema.Namespace = req.Namespace

				logger.V(1).Info("Creating APIResourceSchema", "name", schema.Name, "namespace", schema.Namespace)
				if apiResourceSchema, err = r.createAPIResourceSchema(ctx, schema); err != nil {
					return err
				}
			case err != nil:
				return err
			}

			if _, err := r.getServiceExport(req.Namespace, name); err != nil && !apierrors.IsNotFound(err) {
				return err
			} else if err == nil {
				continue
			}

			hash := helpers.APIResourceSchemaCRDSpecHash(&apiResourceSchema.Spec.APIResourceSchemaCRDSpec)
			export := &kubebindv1alpha2.APIServiceExport{
				ObjectMeta: metav1.ObjectMeta{
					Name:      req.Name,
					Namespace: req.Namespace,
					Annotations: map[string]string{
						kubebindv1alpha2.SourceSpecHashAnnotationKey: hash,
					},
				},
				Spec: kubebindv1alpha2.APIServiceExportSpec{
					Resources: []kubebindv1alpha2.APIResourceSchemaReference{
						{
							Type: "APIResourceSchema",
							Name: apiResourceSchema.Name,
						},
					},
					InformerScope: r.informerScope,
				},
			}
			if apiResourceSchema.Spec.Scope == apiextensionsv1.ClusterScoped {
				export.Spec.ClusterScopedIsolation = r.clusterScopedIsolation
			}

			logger.V(1).Info("Creating APIServiceExport", "name", export.Name, "namespace", export.Namespace)
			if _, err = r.createServiceExport(ctx, export); err != nil {
				return err
			}
		}

		if !failure {
			conditions.MarkTrue(req, kubebindv1alpha2.APIServiceExportRequestConditionExportsReady)
			req.Status.Phase = kubebindv1alpha2.APIServiceExportRequestPhaseSucceeded
			return nil
		}

		if time.Since(req.CreationTimestamp.Time) > time.Minute {
			req.Status.Phase = kubebindv1alpha2.APIServiceExportRequestPhaseFailed
			req.Status.TerminalMessage = conditions.GetMessage(req, kubebindv1alpha2.APIServiceExportRequestConditionExportsReady)
		}

		return nil
	}

	if time.Since(req.CreationTimestamp.Time) > 10*time.Minute {
		logger.Info("Deleting service binding request %s/%s", req.Namespace, req.Name, "reason", "timeout", "age", time.Since(req.CreationTimestamp.Time))
		return r.deleteServiceExportRequest(ctx, req.Namespace, req.Name)
	}

	return nil
}
