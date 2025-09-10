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

package resources

import (
	"context"
	"slices"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func CreateServiceAccount(ctx context.Context, client client.Client, ns, name string) (*corev1.ServiceAccount, error) {
	logger := klog.FromContext(ctx)

	var sa corev1.ServiceAccount
	err := client.Get(ctx, types.NamespacedName{Namespace: ns, Name: name}, &sa)
	if err != nil {
		if errors.IsNotFound(err) {
			sa = corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: ns,
				},
			}

			logger.Info("Creating service account", "name", sa.Name)
			err = client.Create(ctx, &sa)
			return &sa, err
		}
	}

	return &sa, err
}

// EnsureBinderClusterRole ensures that the binder cluster role is present in the cluster. This runs multiple times on bind.
// This role is generic role to allowing generic schema operations and lifecycle. There is separate role for permission claims.
func EnsureBinderClusterRole(ctx context.Context, client client.Client) error {
	logger := klog.FromContext(ctx)

	// Define the ClusterRole rules based on the YAML specification
	clusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "kube-binder",
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"kube-bind.io"},
				Resources: []string{"apiserviceexportrequests"},
				Verbs:     []string{"create", "delete", "patch", "update", "get", "list", "watch"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"namespaces"},
				Verbs:     []string{"get"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"secrets"},
				Verbs:     []string{"get", "watch", "list"},
			},
			{
				APIGroups: []string{"kube-bind.io"},
				Resources: []string{"clusterbindings"},
				Verbs:     []string{"get", "watch", "list"},
			},
			{
				APIGroups: []string{"kube-bind.io"},
				Resources: []string{"clusterbindings/status"},
				Verbs:     []string{"get", "patch", "update"},
			},
			{
				APIGroups: []string{"kube-bind.io"},
				Resources: []string{"apiserviceexports"},
				Verbs:     []string{"get", "watch", "list"},
			},
			{
				APIGroups: []string{"kube-bind.io"},
				Resources: []string{"apiserviceexports/status"},
				Verbs:     []string{"get", "patch", "update"},
			},
			{
				APIGroups: []string{"kube-bind.io"},
				Resources: []string{"apiservicenamespaces"},
				Verbs:     []string{"create", "delete", "patch", "update", "get", "list", "watch"},
			},
			{
				APIGroups: []string{"kube-bind.io"},
				Resources: []string{"boundschemas"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{"kube-bind.io"},
				Resources: []string{"boundschemas/status"},
				Verbs:     []string{"get", "list", "patch", "update"},
			},
		},
	}

	// Try to get existing ClusterRole
	var existing rbacv1.ClusterRole
	err := client.Get(ctx, types.NamespacedName{Name: "kube-binder"}, &existing)
	if err != nil {
		if errors.IsNotFound(err) {
			// Create new ClusterRole
			logger.Info("Creating kube-binder ClusterRole")
			return client.Create(ctx, clusterRole)
		}
		return err
	}

	// Update existing ClusterRole if rules have changed
	if !rulesEqual(existing.Rules, clusterRole.Rules) {
		logger.Info("Updating kube-binder ClusterRole")
		existing.Rules = clusterRole.Rules
		return client.Update(ctx, &existing)
	}

	logger.V(2).Info("kube-binder ClusterRole already exists and is up to date")
	return nil
}

// rulesEqual compares two PolicyRule slices for equality.
func rulesEqual(a, b []rbacv1.PolicyRule) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if !policyRuleEqual(a[i], b[i]) {
			return false
		}
	}
	return true
}

// policyRuleEqual compares two PolicyRule structs for equality.
func policyRuleEqual(a, b rbacv1.PolicyRule) bool {
	if !slices.Equal(a.APIGroups, b.APIGroups) {
		return false
	}
	if !slices.Equal(a.Resources, b.Resources) {
		return false
	}
	if !slices.Equal(a.Verbs, b.Verbs) {
		return false
	}
	if !slices.Equal(a.ResourceNames, b.ResourceNames) {
		return false
	}
	if !slices.Equal(a.NonResourceURLs, b.NonResourceURLs) {
		return false
	}
	return true
}
