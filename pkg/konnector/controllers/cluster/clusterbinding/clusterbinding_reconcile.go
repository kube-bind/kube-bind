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

package clusterbinding

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/tools/cache"
	componentbaseversion "k8s.io/component-base/version"
	"k8s.io/klog/v2"

	"github.com/kube-bind/kube-bind/pkg/version"
	kubebindv1alpha2 "github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2"
	conditionsapi "github.com/kube-bind/kube-bind/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
	"github.com/kube-bind/kube-bind/sdk/apis/third_party/conditions/util/conditions"
)

type reconciler struct {
	// consumerSecretRefKey is the namespace/name value of the APIServiceBinding kubeconfig secret reference.
	consumerSecretRefKey string
	providerNamespace    string
	heartbeatInterval    time.Duration

	getProviderSecret    func() (*corev1.Secret, error)
	getConsumerSecret    func() (*corev1.Secret, error)
	updateConsumerSecret func(ctx context.Context, secret *corev1.Secret) (*corev1.Secret, error)
	createConsumerSecret func(ctx context.Context, secret *corev1.Secret) (*corev1.Secret, error)
}

func (r *reconciler) reconcile(ctx context.Context, binding *kubebindv1alpha2.ClusterBinding) error {
	var errs []error

	if err := r.ensureConsumerSecret(ctx, binding); err != nil {
		errs = append(errs, err)
	}

	if err := r.ensureHeartbeat(ctx, binding); err != nil {
		errs = append(errs, err)
	}

	if err := r.ensureKonnectorVersion(ctx, binding); err != nil {
		errs = append(errs, err)
	}

	conditions.SetSummary(binding)

	return utilerrors.NewAggregate(errs)
}

func (r *reconciler) ensureHeartbeat(_ context.Context, binding *kubebindv1alpha2.ClusterBinding) error {
	binding.Status.HeartbeatInterval.Duration = r.heartbeatInterval
	if now := time.Now(); binding.Status.LastHeartbeatTime.IsZero() || now.After(binding.Status.LastHeartbeatTime.Add(r.heartbeatInterval/2)) {
		binding.Status.LastHeartbeatTime.Time = now
	}

	return nil
}

func (r *reconciler) ensureConsumerSecret(ctx context.Context, binding *kubebindv1alpha2.ClusterBinding) error {
	logger := klog.FromContext(ctx)

	providerSecret, err := r.getProviderSecret()
	if err != nil && !errors.IsNotFound(err) {
		return err
	} else if errors.IsNotFound(err) {
		conditions.MarkFalse(
			binding,
			kubebindv1alpha2.ClusterBindingConditionSecretValid,
			"ProviderSecretNotFound",
			conditionsapi.ConditionSeverityWarning,
			"Provider secret %s/%s not found",
			binding.Namespace, binding.Spec.KubeconfigSecretRef.Name,
		)
		return nil
	}

	if _, found := providerSecret.Data[binding.Spec.KubeconfigSecretRef.Key]; !found {
		conditions.MarkFalse(
			binding,
			kubebindv1alpha2.ClusterBindingConditionSecretValid,
			"ProviderSecretInvalid",
			conditionsapi.ConditionSeverityWarning,
			"Provider secret %s/%s is missing %q string key.",
			r.providerNamespace,
			binding.Spec.KubeconfigSecretRef.Name,
			binding.Spec.KubeconfigSecretRef.Key,
		)
		return nil
	}

	consumerSecret, err := r.getConsumerSecret()
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	if consumerSecret == nil {
		ns, name, err := cache.SplitMetaNamespaceKey(r.consumerSecretRefKey)
		if err != nil {
			return err
		}
		consumerSecret := corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      ns,
				Namespace: name,
			},
			Data: providerSecret.Data,
			Type: providerSecret.Type,
		}
		logger.V(2).Info("Creating consumer secret", "namespace", ns, "name", name)
		if _, err := r.createConsumerSecret(ctx, &consumerSecret); err != nil {
			return err
		}
	} else {
		consumerSecret.Data = providerSecret.Data
		consumerSecret.Type = providerSecret.Type

		logger.V(2).Info("Updating consumer secret", "namespace", consumerSecret.Namespace, "name", consumerSecret.Name)
		if _, err := r.updateConsumerSecret(ctx, consumerSecret); err != nil {
			return err
		}

		// TODO: create events
	}

	conditions.MarkTrue(
		binding,
		kubebindv1alpha2.ClusterBindingConditionSecretValid,
	)

	return nil
}

func (r *reconciler) ensureKonnectorVersion(_ context.Context, binding *kubebindv1alpha2.ClusterBinding) error {
	gitVersion := componentbaseversion.Get().GitVersion
	ver, err := version.BinaryVersion(gitVersion)
	if err != nil {
		binding.Status.KonnectorVersion = "unknown"

		conditions.MarkFalse(
			binding,
			kubebindv1alpha2.ClusterBindingConditionValidVersion,
			"ParseError",
			conditionsapi.ConditionSeverityWarning,
			"Konnector binary version string %q cannot be parsed: %v",
			componentbaseversion.Get().GitVersion,
			err,
		)
		return nil
	}

	binding.Status.KonnectorVersion = ver

	conditions.MarkTrue(
		binding,
		kubebindv1alpha2.ClusterBindingConditionValidVersion,
	)

	return nil
}
