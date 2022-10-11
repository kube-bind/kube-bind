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

package servicenamespace

import (
	"context"
	"fmt"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kubebindv1alpha1 "github.com/kube-bind/kube-bind/pkg/apis/kubebind/v1alpha1"
)

type reconciler struct {
	getNamespace    func(name string) (*corev1.Namespace, error)
	createNamespace func(ctx context.Context, ns *corev1.Namespace) (*corev1.Namespace, error)
	deleteNamespace func(ctx context.Context, name string) error

	getServiceNamespace func(ns, name string) (*kubebindv1alpha1.ServiceNamespace, error)

	getClusterBinding func(ns string) (*kubebindv1alpha1.ClusterBinding, error)

	getRole    func(ns, name string) (*rbacv1.Role, error)
	createRole func(ctx context.Context, cr *rbacv1.Role) (*rbacv1.Role, error)
	updateRole func(ctx context.Context, cr *rbacv1.Role) (*rbacv1.Role, error)

	getRoleBinding    func(ns, name string) (*rbacv1.RoleBinding, error)
	createRoleBinding func(ctx context.Context, crb *rbacv1.RoleBinding) (*rbacv1.RoleBinding, error)
	updateRoleBinding func(ctx context.Context, cr *rbacv1.RoleBinding) (*rbacv1.RoleBinding, error)

	listServiceExports func(ns string) ([]*kubebindv1alpha1.ServiceExport, error)
}

func (c *reconciler) reconcile(ctx context.Context, sns *kubebindv1alpha1.ServiceNamespace) error {
	var ns *corev1.Namespace
	nsName := sns.Namespace + "-" + sns.Name
	if sns.Status.Namespace != "" {
		nsName = sns.Status.Namespace
		ns, _ = c.getNamespace(nsName) // golint:errcheck
	}
	if ns == nil {
		ns = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: nsName,
				Annotations: map[string]string{
					serviceNamespaceAnnotationKey: sns.Namespace + "/" + sns.Name,
				},
			},
		}
		if _, err := c.createNamespace(ctx, ns); err != nil && !errors.IsAlreadyExists(err) {
			return fmt.Errorf("failed to create namespace %q: %w", nsName, err)
		}
	}

	if err := c.ensureRBACRole(ctx, nsName, sns); err != nil {
		return fmt.Errorf("failed to ensure RBAC: %w", err)
	}

	if err := c.ensureRBACRoleBinding(ctx, nsName, sns); err != nil {
		return fmt.Errorf("failed to ensure RBAC: %w", err)
	}

	if sns.Status.Namespace != nsName {
		sns.Status.Namespace = nsName
	}

	return nil
}

func (c *reconciler) ensureRBACRole(ctx context.Context, ns string, sns *kubebindv1alpha1.ServiceNamespace) error {
	objName := sns.Namespace
	role, err := c.getRole(ns, objName)
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to get role %s/%s: %w", ns, objName, err)
	}

	exports, err := c.listServiceExports(ns)
	if err != nil {
		return fmt.Errorf("failed to list service exports: %w", err)
	}
	expected := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      objName,
			Namespace: ns,
		},
	}
	for _, export := range exports {
		for _, resource := range export.Spec.Resources {
			expected.Rules = append(expected.Rules, rbacv1.PolicyRule{
				APIGroups: []string{resource.Group},
				Resources: []string{resource.Resource},
				Verbs:     []string{"get", "list", "watch", "update", "patch", "delete", "create"},
			})
		}
	}

	if role == nil {
		if _, err := c.createRole(ctx, expected); err != nil {
			return fmt.Errorf("failed to create role %s/%s: %w", ns, objName, err)
		}
	} else if !reflect.DeepEqual(role.Rules, expected.Rules) {
		if _, err := c.updateRole(ctx, expected); err != nil {
			return fmt.Errorf("failed to create role %s/%s: %w", ns, objName, err)
		}
	}

	return nil
}

func (c *reconciler) ensureRBACRoleBinding(ctx context.Context, ns string, sns *kubebindv1alpha1.ServiceNamespace) error {
	objName := sns.Namespace
	binding, err := c.getRoleBinding(ns, objName)
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to get role binding %s/%s: %w", ns, objName, err)
	}

	cluster, err := c.getClusterBinding(sns.Namespace)
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to get cluster binding %s/%s: %w", sns.Namespace, "cluster", err)
	} else if errors.IsNotFound(err) {
		return nil // no role binding without cluster binding
	}

	expected := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      objName,
			Namespace: ns,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Namespace: sns.Namespace,
				Name:      cluster.Spec.KubeconfigSecretRef.Name, // this is an example-backend invariant that the service account is equally named
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "Role",
			Name:     objName,
			APIGroup: "rbac.authorization.k8s.io",
		},
	}

	if binding == nil {
		if _, err := c.createRoleBinding(ctx, expected); err != nil {
			return fmt.Errorf("failed to create role binding %s/%s: %w", ns, objName, err)
		}
	} else if !reflect.DeepEqual(binding.Subjects, expected.Subjects) || !reflect.DeepEqual(binding.RoleRef, expected.RoleRef) {
		if _, err := c.updateRoleBinding(ctx, expected); err != nil {
			return fmt.Errorf("failed to create role binding %s/%s: %w", ns, objName, err)
		}
	}

	return nil
}
