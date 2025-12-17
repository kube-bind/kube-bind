/*
Copyright 2025 The Kube Bind Authors.

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

package cluster

import (
	"context"

	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	kubebindv1alpha2 "github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2"
)

type reconciler struct {
	allowedGroups []string
	allowedUsers  []string
}

func (r *reconciler) reconcile(ctx context.Context, client client.Client, _ cache.Cache, cluster *kubebindv1alpha2.Cluster) error {
	var errs []error

	if err := r.ensureOIDCRBAC(ctx, client, cluster); err != nil {
		errs = append(errs, err)
	}

	return utilerrors.NewAggregate(errs)
}

func (r *reconciler) ensureOIDCRBAC(ctx context.Context, client client.Client, cluster *kubebindv1alpha2.Cluster) error {
	if err := r.ensureUserClusterRole(ctx, client, cluster); err != nil {
		return err
	}
	return r.ensureUserClusterRoleBinding(ctx, client, cluster)
}

func (r *reconciler) ensureUserClusterRole(ctx context.Context, client client.Client, cluster *kubebindv1alpha2.Cluster) error {
	clusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "kube-bind-oidc-user",
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"kube-bind.io"},
				Resources: []string{"*"},
				Verbs:     []string{"bind"},
			},
			{
				NonResourceURLs: []string{"/", "/api", "/api/*", "/apis", "/apis/*"},
				Verbs:           []string{"access"},
			},
		},
	}

	if err := controllerutil.SetControllerReference(cluster, clusterRole, client.Scheme()); err != nil {
		return err
	}

	var existing rbacv1.ClusterRole
	err := client.Get(ctx, types.NamespacedName{Name: "kube-bind-oidc-user"}, &existing)
	if err != nil {
		if errors.IsNotFound(err) {
			return client.Create(ctx, clusterRole)
		}
		return err
	}

	existing.Rules = clusterRole.Rules
	existing.OwnerReferences = clusterRole.OwnerReferences
	return client.Update(ctx, &existing)
}

func (r *reconciler) ensureUserClusterRoleBinding(ctx context.Context, client client.Client, cluster *kubebindv1alpha2.Cluster) error {
	clusterRoleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "kube-bind-oidc-user",
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "kube-bind-oidc-user",
		},
	}

	for _, group := range r.allowedGroups {
		clusterRoleBinding.Subjects = append(clusterRoleBinding.Subjects, rbacv1.Subject{
			Kind: "Group",
			Name: group,
		})
	}

	for _, user := range r.allowedUsers {
		clusterRoleBinding.Subjects = append(clusterRoleBinding.Subjects, rbacv1.Subject{
			Kind: "User",
			Name: user,
		})
	}

	if err := controllerutil.SetControllerReference(cluster, clusterRoleBinding, client.Scheme()); err != nil {
		return err
	}

	var existing rbacv1.ClusterRoleBinding
	err := client.Get(ctx, types.NamespacedName{Name: "kube-bind-oidc-user"}, &existing)
	if err != nil {
		if errors.IsNotFound(err) {
			return client.Create(ctx, clusterRoleBinding)
		}
		return err
	}

	existing.RoleRef = clusterRoleBinding.RoleRef
	existing.Subjects = clusterRoleBinding.Subjects
	existing.OwnerReferences = clusterRoleBinding.OwnerReferences
	return client.Update(ctx, &existing)
}
