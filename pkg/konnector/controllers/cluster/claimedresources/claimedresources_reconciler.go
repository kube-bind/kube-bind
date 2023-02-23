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

package claimedresources

import (
	"context"
	"reflect"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/klog/v2"

	kubebindv1alpha1 "github.com/kube-bind/kube-bind/pkg/apis/kubebind/v1alpha1"
)

const annotation = "kube-bind.io/claimedresource"

type readReconciler struct {
	getServiceNamespace func(upstreamNamespace string) (*kubebindv1alpha1.APIServiceNamespace, error)
	getProviderObject   func(ns, name string) (*unstructured.Unstructured, error)

	getConsumerObject    func(ctx context.Context, ns, name string) (*unstructured.Unstructured, error)
	updateConsumerObject func(ctx context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error)
	createConsumerObject func(ctx context.Context, ob *unstructured.Unstructured) (*unstructured.Unstructured, error)
	deleteConsumerObject func(ctx context.Context, ns, name string) error
}

// reconcile syncs upstream claimed resources to downstream.
func (r *readReconciler) reconcile(ctx context.Context, upstreamNS, name string) error {
	logger := klog.FromContext(ctx)
	logger = logger.WithValues("name", name, "upstreamNamespace", upstreamNS)

	logger.Info("reconciling object")
	downstreamNS := ""
	if upstreamNS != "" {
		sn, err := r.getServiceNamespace(upstreamNS)
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
		downstreamNS = sn.Name
		logger = logger.WithValues("downstreamNamespace", downstreamNS)
	}

	obj, err := r.getProviderObject(upstreamNS, name)
	if errors.IsNotFound(err) {
		err := r.deleteConsumerObject(ctx, downstreamNS, name)
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	} else if err != nil {
		return err
	}

	if obj.GetDeletionTimestamp() != nil && !obj.GetDeletionTimestamp().IsZero() {
		logger.Info("Deleting downstream object because it has been deleted upstream", "downStreamNamespace", upstreamNS, "downstreamName", obj.GetName())
		if err := r.deleteConsumerObject(ctx, upstreamNS, obj.GetName()); err != nil {
			return err
		}
	}
	// clean up object
	candidate := obj.DeepCopy()
	candidate.SetUID("")
	candidate.SetResourceVersion("")
	candidate.SetNamespace(downstreamNS)
	candidate.SetManagedFields(nil)
	candidate.SetDeletionTimestamp(nil)
	candidate.SetDeletionGracePeriodSeconds(nil)
	candidate.SetOwnerReferences(nil)
	candidate.SetFinalizers(nil)
	candidate.SetAnnotations(map[string]string{
		annotation: "true"},
	)
	candidate.SetNamespace(downstreamNS)

	downstream, err := r.getConsumerObject(ctx, downstreamNS, name)

	if err != nil && !errors.IsNotFound(err) {
		logger.Info("failed to get downstream object", "error", err, "downstreamNamespace", upstreamNS, "downstreamName", obj.GetName())
		return err
	} else if errors.IsNotFound(err) {
		logger.Info("Creating missing downstream object", "downstreamNamespace", upstreamNS, "downstreamName", obj.GetName())
		if _, err := r.createConsumerObject(ctx, candidate); err != nil {
			return err
		}

		return nil
	}

	if !reflect.DeepEqual(candidate, downstream) {
		logger.Info("Updating downstream object data", "downstreamNamespace", upstreamNS, "downstreamName", downstream.GetName())
		if _, err := r.updateConsumerObject(ctx, candidate); err != nil {
			return err
		}
	}

	return nil
}
