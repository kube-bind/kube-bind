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
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/klog/v2"

	"github.com/kube-bind/kube-bind/pkg/konnector/controllers/cluster/serviceexport/isolation"
)

type reconciler struct {
	isolationStrategy isolation.Strategy

	getConsumerObject          func(ns, name string) (*unstructured.Unstructured, error)
	updateConsumerObjectStatus func(ctx context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error)

	deleteProviderObject func(ctx context.Context, ns, name string) error
}

// reconcile syncs upstream status to consumer objects.
func (r *reconciler) reconcile(ctx context.Context, obj *unstructured.Unstructured) error {
	logger := klog.FromContext(ctx)

	// Map the provider object namespace/name to the consumer side. Technically
	// consumerKey should never be nil (for it to be nil, the namespace in the
	// APIServiceNamespace's status must be blank, but it's blank, how could we
	// have found an object in a non-defined namespace)?, but we are on the safe
	// side and handle it anyway.
	consumerKey, err := r.isolationStrategy.ToConsumerKey(types.NamespacedName{
		Namespace: obj.GetNamespace(),
		Name:      obj.GetName(),
	})
	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}

		return nil
	}
	if consumerKey == nil {
		return nil
	}

	logger = logger.WithValues("downstream", consumerKey)

	downstream, err := r.getConsumerObject(consumerKey.Namespace, consumerKey.Name)
	if err != nil {
		if errors.IsNotFound(err) {
			// downstream is gone. Delete upstream too. Note that we cannot rely on the spec controller because
			// due to konnector restart it might have missed the deletion event.
			logger.Info("Deleting upstream object because downstream is gone")
			return r.deleteProviderObject(ctx, obj.GetNamespace(), obj.GetName())
		}

		logger.Error(err, "failed to get downstream object")
		return err
	}

	// let the isolation perform any changes it desires
	if err := r.isolationStrategy.MutateStatus(downstream, *consumerKey); err != nil {
		return err
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
		logger.Info("Updating downstream object status")
		if _, err := r.updateConsumerObjectStatus(ctx, downstream); err != nil {
			return err
		}
	}

	return nil
}
