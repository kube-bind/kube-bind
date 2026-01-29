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

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kubebindv1alpha2 "github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2"
)

type reconciler struct {
	scope     kubebindv1alpha2.InformerScope
	isolation kubebindv1alpha2.Isolation

	getNamespace    func(ctx context.Context, cache cache.Cache, name string) (*corev1.Namespace, error)
	createNamespace func(ctx context.Context, client client.Client, ns *corev1.Namespace) error
	deleteNamespace func(ctx context.Context, client client.Client, name string) error

	getRoleBinding    func(ctx context.Context, cache cache.Cache, ns, name string) (*rbacv1.RoleBinding, error)
	createRoleBinding func(ctx context.Context, client client.Client, crb *rbacv1.RoleBinding) error
	updateRoleBinding func(ctx context.Context, client client.Client, cr *rbacv1.RoleBinding) error
}

func (c *reconciler) reconcile(ctx context.Context, client client.Client, cache cache.Cache, sns *kubebindv1alpha2.APIServiceNamespace) error {
	var ns *corev1.Namespace
	var nsName string
	switch {
	case sns.Status.Namespace != "":
		// use existing namespace from status
		nsName = sns.Status.Namespace
	case c.isolation == kubebindv1alpha2.IsolationNone:
		nsName = sns.Name
	case c.isolation == kubebindv1alpha2.IsolationNamespaced || c.isolation == kubebindv1alpha2.IsolationPrefixed:
		nsName = sns.Namespace + "-" + sns.Name
	default:
		return fmt.Errorf("unknown isolation strategy: %s", c.isolation)
	}
	ns, _ = c.getNamespace(ctx, cache, nsName)

	if ns == nil {
		ns = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: nsName,
				Annotations: map[string]string{
					kubebindv1alpha2.APIServiceNamespaceAnnotationKey: sns.Namespace + "/" + sns.Name,
				},
			},
		}
		if err := c.createNamespace(ctx, client, ns); err != nil && !errors.IsAlreadyExists(err) {
			return fmt.Errorf("failed to create namespace %q: %w", nsName, err)
		}
	}

	if sns.Status.Namespace != nsName {
		sns.Status.Namespace = nsName
	}

	return nil
}
