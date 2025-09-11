/*
Copyright 2023 The Kube Bind Authors.

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
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/klog/v2"

	kubebindv1alpha2 "github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2"
)

const label = "kube-bind.io/owner"

type readReconciler struct {
	getServiceNamespace  func(upstreamNamespace string) (*kubebindv1alpha2.APIServiceNamespace, error)
	getProviderObject    func(ns, name string) (*unstructured.Unstructured, error)
	createProviderObject func(ctx context.Context, obj *unstructured.Unstructured) error
	updateProviderObject func(ctx context.Context, obj *unstructured.Unstructured) error
	deleteProviderObject func(ctx context.Context, ns, name string) error

	getConsumerObject    func(ctx context.Context, ns, name string) (*unstructured.Unstructured, error)
	updateConsumerObject func(ctx context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error)
	createConsumerObject func(ctx context.Context, ob *unstructured.Unstructured) (*unstructured.Unstructured, error)
	deleteConsumerObject func(ctx context.Context, ns, name string) error
}

// reconcile syncs upstream claimed resources to downstream.
func (r *readReconciler) reconcile(ctx context.Context, providerNS, name string) error {
	logger := klog.FromContext(ctx)
	logger = logger.WithValues("name", name, "providerNamespace", providerNS)

	logger.Info("reconciling object")
	consumerNS := ""
	if providerNS != "" {
		sn, err := r.getServiceNamespace(providerNS)
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

		logger = logger.WithValues("providerNamespace", sn.Status.Namespace)
		consumerNS = sn.Name
		logger = logger.WithValues("consumerNamespace", consumerNS)

		ctx = klog.NewContext(ctx, logger)
	}

	providerObj, providerErr := r.getProviderObject(providerNS, name)
	if providerErr != nil && !errors.IsNotFound(providerErr) {
		return providerErr
	}
	consumerObj, consumerErr := r.getConsumerObject(ctx, consumerNS, name)
	if consumerErr != nil && !errors.IsNotFound(consumerErr) {
		return consumerErr
	}

	if errors.IsNotFound(providerErr) && errors.IsNotFound(consumerErr) {
		// Nothing to do
		return nil
	}

	// Determine owner
	owner, err := determineOwner(providerObj, consumerObj)
	if err != nil { // nothing we can do
		logger.Error(err, "could not determine owner")
		return nil
	}
	logger = logger.WithValues("owner", owner)

	switch owner {
	case kubebindv1alpha2.OwnerProvider:
		if errors.IsNotFound(providerErr) {
			err := r.deleteConsumerObject(ctx, consumerNS, name)
			if errors.IsNotFound(err) {
				return nil
			}
			return err
		}
		ownerCandidate := providerObj.DeepCopy()

		// Set owner label if needed
		r.makeProviderOwner(ownerCandidate)
		if !equality.Semantic.DeepEqual(providerObj, ownerCandidate) {
			if err := r.updateProviderObject(ctx, ownerCandidate); err != nil {
				return err
			}
		}

		if errors.IsNotFound(consumerErr) {
			logger.Info("Creating missing downstream object", "downstreamNamespace", providerNS, "downstreamName", providerObj.GetName())

			candidate := candidateFromOwnerObj(consumerNS, providerObj)
			r.makeProviderOwner(candidate)

			if _, err := r.createConsumerObject(ctx, candidate); err != nil {
				return err
			}

			return nil
		}

		if providerObj.GetDeletionTimestamp() != nil && !providerObj.GetDeletionTimestamp().IsZero() {
			logger.Info("Deleting downstream object because it has been deleted upstream", "downStreamNamespace", providerNS, "downstreamName", providerObj.GetName())
			if err := r.deleteConsumerObject(ctx, providerNS, providerObj.GetName()); err != nil {
				return err
			}
		}

		candidate := candidateFromOwnerObj(consumerNS, providerObj)
		current := candidateFromOwnerObj(consumerNS, consumerObj)
		if !equality.Semantic.DeepEqual(candidate, current) {
			logger.Info("Updating downstream object data", "downstreamNamespace", consumerNS, "downstreamName", consumerObj.GetName())
			if _, err := r.updateConsumerObject(ctx, candidate); err != nil {
				logger.Error(err, "error updating consumer object")
				return err
			}
		}

	case kubebindv1alpha2.OwnerConsumer:
		if errors.IsNotFound(consumerErr) {
			logger.Info("Owner copy of the object is gone, deleting downstream object", "name", name, "namespace", providerNS)
			err := r.deleteProviderObject(ctx, providerNS, name)
			if errors.IsNotFound(err) {
				return nil
			}
			return err
		}

		ownerCandidate := consumerObj.DeepCopy()
		r.makeConsumerOwner(ownerCandidate)
		if !equality.Semantic.DeepEqual(consumerObj, ownerCandidate) {
			logger.Info("setting owner annotation for Consumer object")
			if _, err := r.updateConsumerObject(ctx, ownerCandidate); err != nil {
				return err
			}
		}

		candidate := candidateFromOwnerObj(providerNS, ownerCandidate)
		r.makeConsumerOwner(candidate)

		if errors.IsNotFound(providerErr) {
			logger.Info("creating consumer owned object at provider")
			return r.createProviderObject(ctx, candidate)
		}

		providerObj := candidateFromOwnerObj(providerNS, providerObj)
		if !equality.Semantic.DeepEqual(providerObj, candidate) {
			logger.Info("updating consumer owned object at provider")
			return r.updateProviderObject(ctx, candidate)
		}
	}

	return nil
}

func (r readReconciler) makeConsumerOwner(obj *unstructured.Unstructured) {
	a := obj.GetLabels()
	if a == nil {
		a = map[string]string{}
	}
	a[label] = string(kubebindv1alpha2.OwnerConsumer)
	obj.SetLabels(a)
}

func (r readReconciler) makeProviderOwner(obj *unstructured.Unstructured) {
	a := obj.GetLabels()
	if a == nil {
		a = map[string]string{}
	}
	a[label] = string(kubebindv1alpha2.OwnerProvider)
	obj.SetLabels(a)
}

func candidateFromOwnerObj(downstreamNS string, obj *unstructured.Unstructured) *unstructured.Unstructured {
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
	candidate.SetNamespace(downstreamNS)
	candidate.SetCreationTimestamp(v1.Time{})

	labels := map[string]string{}
	for key, label := range obj.GetLabels() {
		if strings.Contains(key, "claimed.internal.apis.kcp.io") {
			continue
		}
		labels[key] = label
	}
	candidate.SetLabels(labels)

	annotations := map[string]string{}
	for key, annotation := range obj.GetAnnotations() {
		if strings.Contains(key, "kcp.io/cluster") {
			continue
		}
		annotations[key] = annotation
	}
	candidate.SetAnnotations(annotations)

	return candidate
}

// determineOwner determines the owner of a resource given at least one object exists either on the
// consumer or provider side
func determineOwner(providerObj, consumerObj *unstructured.Unstructured) (kubebindv1alpha2.Owner, error) {
	if providerObj != nil {
		ownerLabel := providerObj.GetLabels()[label]
		switch ownerLabel {
		case "provider":
			return kubebindv1alpha2.OwnerProvider, nil
		case "consumer":
			return kubebindv1alpha2.OwnerConsumer, nil
		}
		if ownerLabel == "" && consumerObj == nil {
			return kubebindv1alpha2.OwnerProvider, nil
		}
	}

	if consumerObj != nil {
		ownerLabel := consumerObj.GetLabels()[label]
		switch ownerLabel {
		case "provider":
			return kubebindv1alpha2.OwnerProvider, nil
		case "consumer":
			return kubebindv1alpha2.OwnerConsumer, nil
		}
		if ownerLabel == "" && providerObj == nil {
			return kubebindv1alpha2.OwnerConsumer, nil
		}
	}
	return "", fmt.Errorf("unable to determine owner")
}
