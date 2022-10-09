/*
Copyright 2022 The kube bind Authors.

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

package clusterbinding

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	kubebindv1alpha1 "github.com/kube-bind/kube-bind/pkg/apis/kubebind/v1alpha1"
	conditionsapi "github.com/kube-bind/kube-bind/pkg/apis/third_party/conditions/apis/conditions/v1alpha1"
	"github.com/kube-bind/kube-bind/pkg/apis/third_party/conditions/util/conditions"
)

type reconciler struct {
	// consumerSecretRefKey is the namespace/name value of the ServiceBinding kubeconfig secret reference.
	consumerSecretRefKey string
	providerNamespace    string
	heartbeatInterval    time.Duration

	getProviderSecret    func() (*corev1.Secret, error)
	getConsumerSecret    func() (*corev1.Secret, error)
	updateConsumerSecret func(ctx context.Context, secret *corev1.Secret) (*corev1.Secret, error)
	createConsumerSecret func(ctx context.Context, secret *corev1.Secret) (*corev1.Secret, error)
}

func (r *reconciler) reconcile(ctx context.Context, binding *kubebindv1alpha1.ClusterBinding) error {
	var errs []error
	available := true

	if secretAvailable, err := r.ensureSecret(ctx, binding); err != nil {
		errs = append(errs, err)
	} else {
		available = available && secretAvailable
	}

	if secretAvailable, err := r.ensureHeartbeat(ctx, binding); err != nil {
		errs = append(errs, err)
	} else {
		available = available && secretAvailable
	}

	if available {
		conditions.MarkTrue(
			binding,
			kubebindv1alpha1.ClusterBindingConditionAvailable,
		)
	} else {
		conditions.MarkFalse(
			binding,
			kubebindv1alpha1.ClusterBindingConditionAvailable,
			"ClusterBindingNotAvailable",
			conditionsapi.ConditionSeverityError,
			"Some other condition is not True", // TODO: do better aggregation
		)
	}

	return utilerrors.NewAggregate(errs)
}

func (r *reconciler) ensureHeartbeat(ctx context.Context, binding *kubebindv1alpha1.ClusterBinding) (bool, error) {
	binding.Status.HeartbeatInterval.Duration = r.heartbeatInterval
	if now := time.Now(); binding.Status.LastHeartbeatTime.IsZero() || now.After(binding.Status.LastHeartbeatTime.Add(r.heartbeatInterval/2)) {
		binding.Status.LastHeartbeatTime.Time = now
	}

	return true, nil
}

func (r *reconciler) ensureSecret(ctx context.Context, binding *kubebindv1alpha1.ClusterBinding) (bool, error) {
	logger := klog.FromContext(ctx)

	providerSecret, err := r.getProviderSecret()
	if err != nil && !errors.IsNotFound(err) {
		return false, err
	} else if errors.IsNotFound(err) {
		conditions.MarkFalse(
			binding,
			kubebindv1alpha1.ClusterBindingConditionSecretValid,
			"ProviderSecretNotFound",
			conditionsapi.ConditionSeverityError,
			"Provider secret %s/%s not found",
			binding.Namespace, binding.Spec.KubeconfigSecretRef.Name,
		)
		return false, nil
	}

	if _, found := providerSecret.StringData[binding.Spec.KubeconfigSecretRef.Key]; !found {
		conditions.MarkFalse(
			binding,
			kubebindv1alpha1.ClusterBindingConditionSecretValid,
			"ProviderSecretInvalid",
			conditionsapi.ConditionSeverityError,
			"Provider secret %s/%s is missing %q string key.",
			r.providerNamespace,
			binding.Spec.KubeconfigSecretRef.Name,
			binding.Spec.KubeconfigSecretRef.Key,
		)
		return false, nil
	}

	consumerSecret, err := r.getConsumerSecret()
	if err != nil || !errors.IsNotFound(err) {
		return false, err
	}

	if consumerSecret == nil {
		ns, name, err := cache.SplitMetaNamespaceKey(r.consumerSecretRefKey)
		if err != nil {
			return false, err
		}
		consumerSecret := corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      ns,
				Namespace: name,
			},
			Data:       providerSecret.Data,
			StringData: providerSecret.StringData,
			Type:       providerSecret.Type,
		}
		logger.V(2).Info("Creating consumer secret", "namespace", ns, "name", name)
		if _, err := r.createConsumerSecret(ctx, &consumerSecret); err != nil {
			return false, err
		}
	} else {
		consumerSecret.Data = providerSecret.Data
		consumerSecret.StringData = providerSecret.StringData
		consumerSecret.Type = providerSecret.Type

		logger.V(2).Info("Updating consumer secret", "namespace", consumerSecret.Namespace, "name", consumerSecret.Name)
		if _, err := r.updateConsumerSecret(ctx, consumerSecret); err != nil {
			return false, err
		}

		// TODO: create events
	}

	conditions.MarkTrue(
		binding,
		kubebindv1alpha1.ClusterBindingConditionSecretValid,
	)

	return true, nil
}
