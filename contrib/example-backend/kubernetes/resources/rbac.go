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

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeclient "k8s.io/client-go/kubernetes"
)

func CreateServiceAccount(ctx context.Context, client kubeclient.Interface, ns string) (*corev1.ServiceAccount, error) {
	sa, err := client.CoreV1().ServiceAccounts(ns).Get(ctx, ClusterAdminName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			sa = &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      ClusterAdminName,
					Namespace: ns,
				},
			}

			return client.CoreV1().ServiceAccounts(ns).Create(ctx, sa, metav1.CreateOptions{})
		}
	}

	return sa, err
}

func CreateAdminClusterRoleBinding(ctx context.Context, client kubeclient.Interface, ns string) error {
	if _, err := client.RbacV1().ClusterRoleBindings().Get(ctx, ClusterAdminName, metav1.GetOptions{}); err != nil && !errors.IsNotFound(err) {
		return err
	} else if err == nil {
		return nil
	}

	clusterRoleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: ClusterAdminName,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      ClusterAdminName,
				Namespace: ns,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "cluster-admin",
		},
	}
	if _, err := client.RbacV1().ClusterRoleBindings().Create(ctx, clusterRoleBinding, metav1.CreateOptions{}); err != nil {
		return err
	}

	return nil
}
