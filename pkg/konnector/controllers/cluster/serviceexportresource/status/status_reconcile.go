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
	"reflect"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/klog/v2"

	kubebindv1alpha1 "github.com/kube-bind/kube-bind/pkg/apis/kubebind/v1alpha1"
)

type reconciler struct {
	namespaced bool

	getServiceNamespace func(upstreamNamespace string) (*kubebindv1alpha1.ServiceNamespace, error)

	getConsumerObject          func(ns, name string) (*unstructured.Unstructured, error)
	updateConsumerObjectStatus func(ctx context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error)

	deleteProviderObject func(ctx context.Context, ns, name string) error
}

// reconcile syncs upstream status to consumer objects.
func (r *reconciler) reconcile(ctx context.Context, obj *unstructured.Unstructured) error {
	logger := klog.FromContext(ctx)

	ns := obj.GetNamespace()
	if ns != "" {
		sn, err := r.getServiceNamespace(ns)
		if err != nil && !errors.IsNotFound(err) {
			return err
		} else if errors.IsNotFound(err) {
			runtime.HandleError(err)
			return err // hoping the ServiceNamespace will be created soon. Otherwise, this item goes into backoff.
		}
		if sn.Status.Namespace == "" {
			runtime.HandleError(err)
			return err // hoping the status is set soon.
		}

		logger = logger.WithValues("downstreamNamespace", sn.Status.Namespace)
		ctx = klog.NewContext(ctx, logger)

		// continue with downstream namespace
		ns = sn.Status.Namespace
	}

	downstream, err := r.getConsumerObject(ns, obj.GetName())
	if err != nil && !errors.IsNotFound(err) {
		return err
	} else if errors.IsNotFound(err) {
		// downstream is gone. Delete upstream too. Note that we cannot rely on the spec controller because
		// due to konnector restart it might have missed the deletion event.
		logger.Info("Deleting upstream object because downstream is gone")
		if err := r.deleteProviderObject(ctx, obj.GetNamespace(), obj.GetName()); err != nil {
			return err
		}
	}

	status, found, err := unstructured.NestedFieldNoCopy(obj.Object, "status")
	if err != nil {
		runtime.HandleError(err)
		return nil // nothing we can do here
	}

	orig := downstream.DeepCopy()
	if found {
		if err := unstructured.SetNestedField(downstream.Object, status, "status"); err != nil {
			runtime.HandleError(err)
			return nil // nothing we can do here
		}
	} else {
		unstructured.RemoveNestedField(downstream.Object, "status")
	}
	if !reflect.DeepEqual(orig, downstream) {
		logger.Info("Updating downstream object")
		if _, err := r.updateConsumerObjectStatus(ctx, downstream); err != nil {
			return err
		}
	}

	return nil
}
