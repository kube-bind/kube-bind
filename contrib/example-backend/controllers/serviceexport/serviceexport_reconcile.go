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
	"crypto/sha256"
	"encoding/json"
	"math/big"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/klog/v2"

	kubebindv1alpha1 "github.com/kube-bind/kube-bind/pkg/apis/kubebind/v1alpha1"
	kubebindhelpers "github.com/kube-bind/kube-bind/pkg/apis/kubebind/v1alpha1/helpers"
	conditionsapi "github.com/kube-bind/kube-bind/pkg/apis/third_party/conditions/apis/conditions/v1alpha1"
	"github.com/kube-bind/kube-bind/pkg/apis/third_party/conditions/util/conditions"
)

const (
	soureceSpecHashAnnotationKey = "kube-bind.io/source-spec-hash"
)

type reconciler struct {
	getCRD                      func(name string) (*apiextensionsv1.CustomResourceDefinition, error)
	getServiceExportResource    func(ns, name string) (*kubebindv1alpha1.APIServiceExportResource, error)
	createServiceExportResource func(ctx context.Context, resource *kubebindv1alpha1.APIServiceExportResource) (*kubebindv1alpha1.APIServiceExportResource, error)
	updateServiceExportResource func(ctx context.Context, resource *kubebindv1alpha1.APIServiceExportResource) (*kubebindv1alpha1.APIServiceExportResource, error)
	deleteServiceExportResource func(ctx context.Context, ns, name string) error
}

func (r *reconciler) reconcile(ctx context.Context, export *kubebindv1alpha1.APIServiceExport) error {
	var errs []error

	if err := r.ensureExportResources(ctx, export); err != nil {
		errs = append(errs, err)
	}

	conditions.SetSummary(export)

	return utilerrors.NewAggregate(errs)
}

func (r *reconciler) ensureExportResources(ctx context.Context, export *kubebindv1alpha1.APIServiceExport) error {
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
				logger.V(1).Info("Deleting APIServiceExportResource because CRD is missing")
				if err := r.deleteServiceExportResource(ctx, export.Namespace, name); err != nil && !errors.IsNotFound(err) {
					return err
				}
			}

			if resourceInSync {
				conditions.MarkFalse(
					export,
					kubebindv1alpha1.APIServiceExportConditionResourcesInSync,
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
					kubebindv1alpha1.APIServiceExportConditionResourcesInSync,
					"CustomResourceDefinitionUpdateFailed",
					conditionsapi.ConditionSeverityError,
					"CustomResourceDefinition %s cannot be converted into a APIServiceExportResource: %s",
					name, err,
				)
				resourceInSync = false
			}
			continue
		}
		resource.Namespace = export.Namespace

		if ser == nil {
			// APIServiceExportResource missing
			logger.V(1).Info("Creating APIServiceExportResource")
			resource.Annotations = map[string]string{
				soureceSpecHashAnnotationKey: resourceHash(resource),
			}
			if _, err := r.createServiceExportResource(ctx, resource); err != nil {
				errs = append(errs, err)
				continue
			}
		} else if expected := resourceHash(resource); ser.Annotations[soureceSpecHashAnnotationKey] != expected {
			// both exist, update APIServiceExportResource
			logger.V(1).Info("Updating APIServiceExportResource")
			resource.ObjectMeta = ser.ObjectMeta
			if resource.Annotations == nil {
				resource.Annotations = map[string]string{}
			}
			resource.Annotations[soureceSpecHashAnnotationKey] = expected
			if _, err := r.updateServiceExportResource(ctx, resource); err != nil {
				errs = append(errs, err)
				continue
			}
		}
	}

	if resourceInSync {
		conditions.MarkTrue(export, kubebindv1alpha1.APIServiceExportConditionResourcesInSync)
	}

	return utilerrors.NewAggregate(errs)
}

func resourceHash(obj runtime.Object) string {
	bs, err := json.Marshal(obj)
	if err != nil {
		utilruntime.HandleError(err)
		return ""
	}

	return toSha224Base62(string(bs))
}

func toSha224Base62(s string) string {
	return toBase62(sha256.Sum224([]byte(s)))
}

func toBase62(hash [28]byte) string {
	var i big.Int
	i.SetBytes(hash[:])
	return i.Text(62)
}
