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

package spec

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"

	"github.com/kube-bind/kube-bind/pkg/konnector/controllers/cluster/serviceexport/isolation"
	konnectortypes "github.com/kube-bind/kube-bind/pkg/konnector/types"
	kubebindv1alpha2 "github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2"
)

type reconciler struct {
	isolationStrategy isolation.Strategy
	clusterNamespace  string

	getServiceNamespace    func(name string) (*kubebindv1alpha2.APIServiceNamespace, error)
	createServiceNamespace func(ctx context.Context, sn *kubebindv1alpha2.APIServiceNamespace) (*kubebindv1alpha2.APIServiceNamespace, error)

	getProviderObject    func(ns, name string) (*unstructured.Unstructured, error)
	createProviderObject func(ctx context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error)
	updateProviderObject func(ctx context.Context, obj *unstructured.Unstructured) error
	deleteProviderObject func(ctx context.Context, ns, name string) error

	updateConsumerObject func(ctx context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error)

	requeue func(obj *unstructured.Unstructured, after time.Duration) error
}

// reconcile syncs downstream objects (metadata and spec) with upstream objects.
func (r *reconciler) reconcile(ctx context.Context, obj *unstructured.Unstructured) error {
	logger := klog.FromContext(ctx)

	// Translate the namespace/name from the consumer side to the provider side,
	// potentially creating eventually-consistent resources, hence the returned
	// providerKey can be nil to indicate a requeue is desired.

	providerKey, err := r.isolationStrategy.EnsureProviderKey(ctx, types.NamespacedName{
		Namespace: obj.GetNamespace(),
		Name:      obj.GetName(),
	})
	if err != nil {
		return err
	}
	if providerKey == nil {
		return r.requeue(obj, 1*time.Second)
	}

	upstream, err := r.getProviderObject(providerKey.Namespace, providerKey.Name)
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	} else if apierrors.IsNotFound(err) {
		if !obj.GetDeletionTimestamp().IsZero() {
			logger.V(2).Info("object is already deleting, don't sync")

			if _, err := r.removeDownstreamFinalizer(ctx, obj); err != nil {
				return err
			}

			return nil
		}

		if obj, err = r.ensureDownstreamFinalizer(ctx, obj); err != nil {
			return err
		}

		// clean up object
		upstream = obj.DeepCopy()
		upstream.SetUID("")
		upstream.SetResourceVersion("")
		upstream.SetManagedFields(nil)
		upstream.SetDeletionTimestamp(nil)
		upstream.SetDeletionGracePeriodSeconds(nil)
		upstream.SetOwnerReferences(nil)
		upstream.SetFinalizers(nil)
		unstructured.RemoveNestedField(upstream.Object, "status")

		// Regardless of isolation mode, we always annotate every object with its
		// owning cluster namespace.
		if err := r.setClusterNamespaceAnnotation(upstream); err != nil {
			return err
		}

		// let the isolation perform any changes it desires
		if err := r.isolationStrategy.MutateMetadataAndSpec(upstream, *providerKey); err != nil {
			return err
		}

		logger.Info("Creating upstream object")
		logger.V(4).Info("Upstream object", "object", fmt.Sprintf("%s", upstream.Object))
		if _, err := r.createProviderObject(ctx, upstream); err != nil && !apierrors.IsAlreadyExists(err) {
			return err
		} else if apierrors.IsAlreadyExists(err) {
			logger.Info("Upstream object already exists. Waiting for requeue.") // the upstream object will lead to a requeue
		}
	}

	// here the upstream already exists. Update everything but the status.

	if !obj.GetDeletionTimestamp().IsZero() {
		if !upstream.GetDeletionTimestamp().IsZero() {
			logger.V(2).Info("upstream is already deleting, wait for it")
			return nil // we will get an event when the upstream is deleted
		}

		logger.V(1).Info("object is already deleting downstream, deleting upstream too")
		if err := r.deleteProviderObject(ctx, providerKey.Namespace, providerKey.Name); err != nil && !apierrors.IsNotFound(err) {
			return err
		}

		if _, err := r.removeDownstreamFinalizer(ctx, obj); err != nil {
			return err
		}

		logger.V(2).Info("upstream deleted, finalizer removed in downstream, waiting for downstream deletion to finish")
		return nil // we will get an event when the upstream is deleted
	}

	// (Re)set the annotation if it's missing, abort if for whatever reason the
	// object is annotated with another cluster namespace.
	if err := r.setClusterNamespaceAnnotation(upstream); err != nil {
		return err
	}

	// just in case, checking for finalizer
	if obj, err = r.ensureDownstreamFinalizer(ctx, obj); err != nil {
		return err
	}

	downstreamSpec, foundDownstreamSpec, err := unstructured.NestedFieldNoCopy(obj.Object, "spec")
	if err != nil {
		logger.Error(err, "failed to get downstream spec")
		return nil
	}
	upstreamSpec, _, err := unstructured.NestedFieldNoCopy(upstream.Object, "spec")
	if err != nil {
		logger.Error(err, "failed to get upstream spec")
		return nil
	}
	if reflect.DeepEqual(downstreamSpec, upstreamSpec) {
		return nil // nothing to do
	}

	upstream = upstream.DeepCopy()
	if foundDownstreamSpec {
		if err := unstructured.SetNestedField(upstream.Object, downstreamSpec, "spec"); err != nil {
			bs, err := json.Marshal(downstreamSpec)
			if err != nil {
				logger.Error(err, "failed to marshal downstream spec", "spec", fmt.Sprintf("%s", downstreamSpec))
				return nil // nothing we can do
			}
			logger.Error(err, "failed to set spec", "spec", string(bs))
			return nil // nothing we can do
		}
	} else {
		unstructured.RemoveNestedField(upstream.Object, "spec")
	}

	logger.Info("Updating upstream object")
	return r.updateProviderObject(ctx, upstream)
}

func (r *reconciler) ensureDownstreamFinalizer(ctx context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	logger := klog.FromContext(ctx)

	if !slices.Contains(obj.GetFinalizers(), kubebindv1alpha2.DownstreamFinalizer) {
		logger.V(2).Info("adding finalizer to downstream object")
		obj = obj.DeepCopy()
		obj.SetFinalizers(append(obj.GetFinalizers(), kubebindv1alpha2.DownstreamFinalizer))
		var err error
		if obj, err = r.updateConsumerObject(ctx, obj); err != nil {
			return nil, err
		}
	}

	return obj, nil
}

func (r *reconciler) removeDownstreamFinalizer(ctx context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	logger := klog.FromContext(ctx)

	finalizers := []string{}
	found := false
	for _, f := range obj.GetFinalizers() {
		if f == kubebindv1alpha2.DownstreamFinalizer {
			found = true
			continue
		}
		finalizers = append(finalizers, f)
	}

	if found {
		logger.V(2).Info("removing finalizer from downstream object")
		obj = obj.DeepCopy()
		obj.SetFinalizers(finalizers)
		var err error
		if obj, err = r.updateConsumerObject(ctx, obj); err != nil {
			return nil, err
		}
	}

	return obj, nil
}

func (r *reconciler) setClusterNamespaceAnnotation(obj *unstructured.Unstructured) error {
	annnotations := obj.GetAnnotations()
	existing, annotationExists := annnotations[konnectortypes.ClusterNamespaceAnnotationKey]
	if annotationExists && existing != r.clusterNamespace {
		return errors.New("mismatch between existing cluster namespace and given cluster namespace")
	}

	if !annotationExists {
		if annnotations == nil {
			annnotations = map[string]string{}
		}
		annnotations[konnectortypes.ClusterNamespaceAnnotationKey] = r.clusterNamespace
		obj.SetAnnotations(annnotations)
	}

	return nil
}
