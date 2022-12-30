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

package status

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/klog/v2"

	kubebindv1alpha1 "github.com/kube-bind/kube-bind/pkg/apis/kubebind/v1alpha1"
)

type reconciler struct {
	getServiceNamespace func(upstreamNamespace string) (*kubebindv1alpha1.APIServiceNamespace, error)

	getConsumerObject          func(ns, name string) (*unstructured.Unstructured, error)
	updateConsumerObjectStatus func(ctx context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error)
	updateConsumerObject       func(ctx context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error)
	createConsumerObject       func(ctx context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error)

	deleteProviderObject func(ctx context.Context, ns, name string) error
}

// reconcile syncs upstream status to consumer objects.
func (r *reconciler) reconcile(ctx context.Context, obj *unstructured.Unstructured) error {

	logger := klog.FromContext(ctx)
	fmt.Println("reconciling: ", obj.GetNamespace(), "/", obj.GetName())

	obj = obj.DeepCopy()

	ns := obj.GetNamespace()
	if ns != "" {
		sn, err := r.getServiceNamespace(ns)
		if err != nil && !errors.IsNotFound(err) {
			return err
		} else if errors.IsNotFound(err) {
			runtime.HandleError(err)
			return err // hoping the APIServiceNamespace will be created soon. Otherwise, this item goes into backoff.
		}
		if sn.Status.Namespace == "" {
			runtime.HandleError(err)
			return err // hoping the status is set soon.
		}

		logger = logger.WithValues("upstreamNamespace", sn.Status.Namespace)
		ctx = klog.NewContext(ctx, logger)

		// continue with downstream namespace
		ns = sn.Name
	}
	fmt.Println("service namespace: ", ns)

	if _, found := obj.GetLabels()["provider-created"]; found {
		_, err := r.getConsumerObject(ns, obj.GetName())
		if err == nil {
			downstream, er := r.getConsumerObject(ns, obj.GetName())
			if er != nil {
				return er
			}

			downstreamSpec, _, err := unstructured.NestedFieldNoCopy(downstream.Object, "spec")
			if err != nil {
				logger.Error(err, "failed to get downstream spec")
				return nil
			}
			upstreamSpec, foundUpstreamSpec, err := unstructured.NestedFieldNoCopy(obj.Object, "spec")
			if err != nil {
				logger.Error(err, "failed to get upstream spec")
				return nil
			}

			if foundUpstreamSpec && !reflect.DeepEqual(downstreamSpec, upstreamSpec){
				if err := unstructured.SetNestedField(downstream.Object, upstreamSpec, "spec"); err != nil {
					bs, err := json.Marshal(upstreamSpec)
					if err != nil {
						logger.Error(err, "failed to marshal downstream spec", "spec", fmt.Sprintf("%s", downstreamSpec))
						return nil // nothing we can do
					}
					logger.Error(err, "failed to set spec", "spec", string(bs))
					return nil // nothing we can do
				}
			} else {
				unstructured.RemoveNestedField(downstream.Object, "spec")
			}

			if _, er := r.updateConsumerObject(ctx, downstream); er != nil {
				return er
			}

			// upstream status sync with downstream status

			downstreamStatus, _, err := unstructured.NestedFieldNoCopy(downstream.Object, "status")
			if err != nil {
				logger.Error(err, "failed to get downstream status")
				return nil
			}
			upstreamStatus, foundUpstreamStatus, err := unstructured.NestedFieldNoCopy(obj.Object, "status")
			if err != nil {
				logger.Error(err, "failed to get upstream status")
				return nil
			}

			if foundUpstreamStatus && !reflect.DeepEqual(downstreamStatus, upstreamStatus) {
				if err := unstructured.SetNestedField(downstream.Object, upstreamStatus, "status"); err != nil {
					bs, err := json.Marshal(upstreamStatus)
					if err != nil {
						logger.Error(err, "failed to marshal upstream status", "status", fmt.Sprintf("%s", upstreamStatus))
						return nil // nothing we can do
					}
					logger.Error(err, "failed to set spec", "spec", string(bs))
					return nil // nothing we can do
				}
			} else {
				unstructured.RemoveNestedField(downstream.Object, "status")
			}

			if _, er := r.updateConsumerObjectStatus(ctx, downstream); er != nil {
				return er
			}
		} else if errors.IsNotFound(err) {
			upstream := obj.DeepCopy()
			upstream.SetUID("")
			upstream.SetResourceVersion("")
			upstream.SetNamespace(ns)
			upstream.SetManagedFields(nil)
			upstream.SetDeletionTimestamp(nil)
			upstream.SetDeletionGracePeriodSeconds(nil)
			upstream.SetOwnerReferences(nil)
			upstream.SetFinalizers(nil)
			if _, er := r.createConsumerObject(ctx, upstream); er != nil {
				return er
			}
		} else {
			return err
		}
		return nil
	}

	downstream, err := r.getConsumerObject(ns, obj.GetName())
	if err != nil && !errors.IsNotFound(err) {
		logger.Info("failed to get downstream object", "error", err, "downstreamNamespace", ns, "downstreamName", obj.GetName())
		return err
	} else if errors.IsNotFound(err) {
		// downstream is gone. Delete upstream too. Note that we cannot rely on the spec controller because
		// due to konnector restart it might have missed the deletion event.
		logger.Info("Deleting upstream object because downstream is gone", "downstreamNamespace", ns, "downstreamName", obj.GetName())
		if err := r.deleteProviderObject(ctx, obj.GetNamespace(), obj.GetName()); err != nil {
			return err
		}
		return nil
	}

	orig := downstream
	downstream = downstream.DeepCopy()
	status, found, err := unstructured.NestedFieldNoCopy(obj.Object, "status")
	if err != nil {
		runtime.HandleError(err)
		return nil // nothing we can do here
	}
	if found {
		if err := unstructured.SetNestedField(downstream.Object, status, "status"); err != nil {
			runtime.HandleError(err)
			return nil // nothing we can do here
		}
	} else {
		unstructured.RemoveNestedField(downstream.Object, "status")
	}
	if !reflect.DeepEqual(orig, downstream) {
		logger.Info("Updating downstream object status", "downstreamNamespace", ns, "downstreamName", obj.GetName())
		if _, err := r.updateConsumerObjectStatus(ctx, downstream); err != nil {
			return err
		}
	}

	return nil
}
