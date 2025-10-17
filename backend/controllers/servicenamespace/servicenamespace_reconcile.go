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
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kuberesources "github.com/kube-bind/kube-bind/backend/kubernetes/resources"
	kubebindv1alpha2 "github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2"
)

type reconciler struct {
	scope kubebindv1alpha2.InformerScope

	getNamespace    func(ctx context.Context, cache cache.Cache, name string) (*corev1.Namespace, error)
	createNamespace func(ctx context.Context, client client.Client, ns *corev1.Namespace) error
	deleteNamespace func(ctx context.Context, client client.Client, name string) error

	getRoleBinding    func(ctx context.Context, cache cache.Cache, ns, name string) (*rbacv1.RoleBinding, error)
	createRoleBinding func(ctx context.Context, client client.Client, crb *rbacv1.RoleBinding) error
	updateRoleBinding func(ctx context.Context, client client.Client, cr *rbacv1.RoleBinding) error
}

func (c *reconciler) reconcile(ctx context.Context, client client.Client, cache cache.Cache, sns *kubebindv1alpha2.APIServiceNamespace) error {
	var ns *corev1.Namespace
	nsName := sns.Namespace + "-" + sns.Name
	if sns.Status.Namespace != "" {
		nsName = sns.Status.Namespace
		ns, _ = c.getNamespace(ctx, cache, nsName) // golint:errcheck
	}
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

	if c.scope == kubebindv1alpha2.NamespacedScope {
		if err := c.ensureRBACRoleBinding(ctx, client, cache, nsName, sns); err != nil {
			return fmt.Errorf("failed to ensure RBAC: %w", err)
		}
	}

	if sns.Status.Namespace != nsName {
		sns.Status.Namespace = nsName
	}

	// List APIServiceExports in the namespace
	apiServiceExports, err := c.listAPIServiceExports(ctx, cache, sns.Namespace)
	if err != nil {
		return fmt.Errorf("failed to list APIServiceExports: %w", err)
	}

	for _, export := range apiServiceExports.Items {
		name := fmt.Sprintf("kube-binder-%s-export-%s", sns.Name, export.Name) // per-sns unique name
		permissions := []rbacv1.PolicyRule{}
		for _, claim := range export.Spec.PermissionClaims {
			permissions = append(permissions, rbacv1.PolicyRule{
				APIGroups: []string{claim.Group},
				Resources: []string{claim.Resource},
				// We need list and watch for informers to be able to start. And create to create initial object.
				Verbs: []string{"*"},
			})
		}
		if c.scope == kubebindv1alpha2.ClusterScope {
			role, err := c.getPermissionClaimsClusterRole(ctx, cache, name)
			if err != nil && !errors.IsNotFound(err) {
				return fmt.Errorf("failed to get ClusterRole %s: %w", name, err)
			}
			if role == nil {
				role = &rbacv1.ClusterRole{
					ObjectMeta: metav1.ObjectMeta{
						Name: name,
					},
					Rules: permissions,
				}
				// Create new ClusterRole
				if err := client.Create(ctx, role); !errors.IsAlreadyExists(err) {
					return fmt.Errorf("failed to create ClusterRole %s: %w", name, err)
				}
			} else {
				role.Rules = permissions
				if err := client.Update(ctx, role); err != nil {
					return fmt.Errorf("failed to update ClusterRole %s: %w", name, err)
				}
			}

			clusterBinding, err := c.getPermissionClaimsClusterRoleBinding(ctx, cache, name)
			if err != nil && !errors.IsNotFound(err) {
				return fmt.Errorf("failed to get ClusterRoleBinding %s: %w", name, err)
			}
			if clusterBinding == nil {
				clusterBinding = &rbacv1.ClusterRoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name: name,
					},
					Subjects: []rbacv1.Subject{
						{
							Kind:      "ServiceAccount",
							Namespace: sns.Namespace,
							Name:      kuberesources.ServiceAccountName,
						},
					},
					RoleRef: rbacv1.RoleRef{
						Kind:     "ClusterRole",
						Name:     name,
						APIGroup: "rbac.authorization.k8s.io",
					},
				}
				if err := client.Create(ctx, clusterBinding); err != nil {
					return fmt.Errorf("failed to create ClusterRoleBinding %s: %w", name, err)
				}
			} else {
				expectedSubjects := []rbacv1.Subject{{
					Kind:      "ServiceAccount",
					Namespace: sns.Namespace,
					Name:      kuberesources.ServiceAccountName,
				}}
				expectedRef := rbacv1.RoleRef{Kind: "ClusterRole", Name: name, APIGroup: "rbac.authorization.k8s.io"}
				if !reflect.DeepEqual(clusterBinding.Subjects, expectedSubjects) || !reflect.DeepEqual(clusterBinding.RoleRef, expectedRef) {
					rb := clusterBinding.DeepCopy()
					rb.Subjects = expectedSubjects
					rb.RoleRef = expectedRef
					if err := client.Update(ctx, rb); err != nil {
						return fmt.Errorf("failed to update ClusterRoleBinding %s: %w", name, err)
					}
				}
			}
		} else {
			role, err := c.getPermissionClaimsRole(ctx, cache, sns.Status.Namespace, name)
			if err != nil && !errors.IsNotFound(err) {
				return fmt.Errorf("failed to get Role %s: %w", name, err)
			}
			if role == nil {
				role := &rbacv1.Role{
					ObjectMeta: metav1.ObjectMeta{
						Name:      name,
						Namespace: sns.Status.Namespace,
					},
					Rules: permissions,
				}
				// Create new Role
				if err := client.Create(ctx, role); err != nil {
					return fmt.Errorf("failed to create Role %s: %w", name, err)
				}
			} else {
				role.Rules = permissions
				if err := client.Update(ctx, role); err != nil {
					return fmt.Errorf("failed to update Role %s: %w", name, err)
				}
			}

			rolebinding, err := c.getPermissionClaimsRoleBinding(ctx, cache, sns.Status.Namespace, name)
			if err != nil && !errors.IsNotFound(err) {
				return fmt.Errorf("failed to get RoleBinding %s: %w", name, err)
			}

			if rolebinding == nil {
				rolebinding := &rbacv1.RoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:      name,
						Namespace: sns.Status.Namespace,
					},
					Subjects: []rbacv1.Subject{
						{
							Kind:      "ServiceAccount",
							Namespace: sns.Namespace,
							Name:      kuberesources.ServiceAccountName,
						},
					},
					RoleRef: rbacv1.RoleRef{
						Kind:     "Role",
						Name:     name,
						APIGroup: "rbac.authorization.k8s.io",
					},
				}
				if err := client.Create(ctx, rolebinding); err != nil {
					return fmt.Errorf("failed to create RoleBinding %s: %w", name, err)
				}
			} else {
				expectedSubjects := []rbacv1.Subject{{
					Kind:      "ServiceAccount",
					Namespace: sns.Namespace,
					Name:      kuberesources.ServiceAccountName,
				}}
				if !reflect.DeepEqual(rolebinding.Subjects, expectedSubjects) || rolebinding.RoleRef.Kind != "Role" || rolebinding.RoleRef.Name != name || rolebinding.RoleRef.APIGroup != "rbac.authorization.k8s.io" {
					rb := rolebinding.DeepCopy()
					rb.Subjects = expectedSubjects
					rb.RoleRef = rbacv1.RoleRef{Kind: "Role", Name: name, APIGroup: "rbac.authorization.k8s.io"}
					if err := client.Update(ctx, rb); err != nil {
						return fmt.Errorf("failed to update RoleBinding %s: %w", name, err)
					}
				}
			}
		}
	}

	return nil
}

func (c *reconciler) ensureRBACRoleBinding(ctx context.Context, client client.Client, cache cache.Cache, ns string, sns *kubebindv1alpha2.APIServiceNamespace) error {
	objName := "kube-binder"
	binding, err := c.getRoleBinding(ctx, cache, ns, objName)
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to get role binding %s/%s: %w", ns, objName, err)
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
				Name:      kuberesources.ServiceAccountName,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "ClusterRole",
			Name:     "kube-binder-" + sns.Namespace,
			APIGroup: "rbac.authorization.k8s.io",
		},
	}

	if binding == nil {
		if err := c.createRoleBinding(ctx, client, expected); err != nil {
			return fmt.Errorf("failed to create role binding %s/%s: %w", ns, objName, err)
		}
	} else if !reflect.DeepEqual(binding.Subjects, expected.Subjects) || !reflect.DeepEqual(binding.RoleRef, expected.RoleRef) {
		binding = binding.DeepCopy()
		binding.Subjects = expected.Subjects
		binding.RoleRef = expected.RoleRef
		if err := c.updateRoleBinding(ctx, client, binding); err != nil {
			return fmt.Errorf("failed to create role binding %s/%s: %w", ns, objName, err)
		}
	}

	return nil
}

func (c *reconciler) getPermissionClaimsClusterRole(ctx context.Context, cache cache.Cache, name string) (*rbacv1.ClusterRole, error) {
	var role rbacv1.ClusterRole
	key := types.NamespacedName{Name: name}
	if err := cache.Get(ctx, key, &role); err != nil {
		return nil, err
	}
	return &role, nil
}

func (c *reconciler) getPermissionClaimsRole(ctx context.Context, cache cache.Cache, namespace, name string) (*rbacv1.Role, error) {
	var role rbacv1.Role
	key := types.NamespacedName{Namespace: namespace, Name: name}
	if err := cache.Get(ctx, key, &role); err != nil {
		return nil, err
	}
	return &role, nil
}

func (c *reconciler) getPermissionClaimsClusterRoleBinding(ctx context.Context, cache cache.Cache, name string) (*rbacv1.ClusterRoleBinding, error) {
	var roleBinding rbacv1.ClusterRoleBinding
	key := types.NamespacedName{Name: name}
	if err := cache.Get(ctx, key, &roleBinding); err != nil {
		return nil, err
	}
	return &roleBinding, nil
}

func (c *reconciler) getPermissionClaimsRoleBinding(ctx context.Context, cache cache.Cache, namespace, name string) (*rbacv1.RoleBinding, error) {
	var roleBinding rbacv1.RoleBinding
	key := types.NamespacedName{Name: name, Namespace: namespace}
	if err := cache.Get(ctx, key, &roleBinding); err != nil {
		return nil, err
	}
	return &roleBinding, nil
}

func (c *reconciler) listAPIServiceExports(ctx context.Context, cache cache.Cache, namespace string) (*kubebindv1alpha2.APIServiceExportList, error) {
	exports := &kubebindv1alpha2.APIServiceExportList{}
	if err := cache.List(ctx, exports, client.InNamespace(namespace)); err != nil {
		return nil, err
	}
	return exports, nil
}
