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

package serviceexportrbac

import (
	"context"
	"maps"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kubebindv1alpha2 "github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2"
)

func Test_reconciler_reconcile(t *testing.T) {
	type state struct {
		clusterRoles        map[string]*rbacv1.ClusterRole
		clusterRoleBindings map[string]*rbacv1.ClusterRoleBinding
		roles               map[string]*rbacv1.Role
		roleBindings        map[string]*rbacv1.RoleBinding
	}
	normalizeState := func(st *state) {
		if st.clusterRoles == nil {
			st.clusterRoles = make(map[string]*rbacv1.ClusterRole)
		}
		if st.clusterRoleBindings == nil {
			st.clusterRoleBindings = make(map[string]*rbacv1.ClusterRoleBinding)
		}
		if st.clusterRoleBindings == nil {
			st.clusterRoleBindings = make(map[string]*rbacv1.ClusterRoleBinding)
		}
		if st.roles == nil {
			st.roles = make(map[string]*rbacv1.Role)
		}
		if st.roleBindings == nil {
			st.roleBindings = make(map[string]*rbacv1.RoleBinding)
		}
	}

	newReconcilerWithState := func(scope kubebindv1alpha2.InformerScope, namespaces map[string]*corev1.Namespace, apiServiceNamespaces map[types.NamespacedName]*kubebindv1alpha2.APIServiceNamespace, st *state) reconciler {
		return reconciler{
			scope: scope,
			listServiceNamespaces: func(ctx context.Context, _ cache.Cache, namespace string) ([]*kubebindv1alpha2.APIServiceNamespace, error) {
				return slices.Collect(maps.Values(apiServiceNamespaces)), nil
			},
			getClusterRole: func(ctx context.Context, _ cache.Cache, name string) (*rbacv1.ClusterRole, error) {
				clusterRole, ok := st.clusterRoles[name]
				if !ok {
					return nil, apierrors.NewNotFound(rbacv1.Resource("clusterroles"), name)
				}
				return clusterRole, nil
			},
			createClusterRole: func(ctx context.Context, _ client.Client, binding *rbacv1.ClusterRole) error {
				_, ok := st.clusterRoles[binding.Name]
				if ok {
					return apierrors.NewAlreadyExists(rbacv1.Resource("clusterroles"), binding.Name)
				}
				st.clusterRoles[binding.Name] = binding
				return nil
			},
			updateClusterRole: func(ctx context.Context, _ client.Client, binding *rbacv1.ClusterRole) error {
				_, ok := st.clusterRoles[binding.Name]
				if !ok {
					return apierrors.NewNotFound(rbacv1.Resource("clusterroles"), binding.Name)
				}
				st.clusterRoles[binding.Name] = binding
				return nil
			},
			getRole: func(ctx context.Context, _ cache.Cache, namespace, name string) (*rbacv1.Role, error) {
				key := types.NamespacedName{Namespace: namespace, Name: name}.String()
				role, ok := st.roles[key]
				if !ok {
					return nil, apierrors.NewNotFound(rbacv1.Resource("roles"), key)
				}
				return role, nil
			},
			createRole: func(ctx context.Context, _ client.Client, role *rbacv1.Role) error {
				key := types.NamespacedName{Namespace: role.Namespace, Name: role.Name}.String()
				_, ok := st.roles[key]
				if ok {
					return apierrors.NewAlreadyExists(rbacv1.Resource("roles"), key)
				}
				st.roles[key] = role
				return nil
			},
			updateRole: func(ctx context.Context, _ client.Client, role *rbacv1.Role) error {
				key := types.NamespacedName{Namespace: role.Namespace, Name: role.Name}.String()
				_, ok := st.roles[key]
				if !ok {
					return apierrors.NewNotFound(rbacv1.Resource("roles"), key)
				}
				st.roles[key] = role
				return nil
			},
			getClusterRoleBinding: func(ctx context.Context, _ cache.Cache, name string) (*rbacv1.ClusterRoleBinding, error) {
				clusterRoleBinding, ok := st.clusterRoleBindings[name]
				if !ok {
					return nil, apierrors.NewNotFound(rbacv1.Resource("clusterrolebindings"), name)
				}
				return clusterRoleBinding, nil
			},
			createClusterRoleBinding: func(ctx context.Context, _ client.Client, binding *rbacv1.ClusterRoleBinding) error {
				_, ok := st.clusterRoleBindings[binding.Name]
				if ok {
					return apierrors.NewAlreadyExists(rbacv1.Resource("clusterrolebindings"), binding.Name)
				}
				st.clusterRoleBindings[binding.Name] = binding
				return nil
			},
			updateClusterRoleBinding: func(ctx context.Context, _ client.Client, binding *rbacv1.ClusterRoleBinding) error {
				_, ok := st.clusterRoleBindings[binding.Name]
				if !ok {
					return apierrors.NewNotFound(rbacv1.Resource("clusterrolebindings"), binding.Name)
				}
				st.clusterRoleBindings[binding.Name] = binding
				return nil
			},
			getRoleBinding: func(ctx context.Context, _ cache.Cache, ns, name string) (*rbacv1.RoleBinding, error) {
				key := types.NamespacedName{Namespace: ns, Name: name}.String()
				roleBinding, ok := st.roleBindings[key]
				if !ok {
					return nil, apierrors.NewNotFound(rbacv1.Resource("rolebindings"), key)
				}
				return roleBinding, nil
			},
			createRoleBinding: func(ctx context.Context, _ client.Client, binding *rbacv1.RoleBinding) error {
				key := types.NamespacedName{Namespace: binding.Namespace, Name: binding.Name}.String()
				_, ok := st.roleBindings[key]
				if ok {
					return apierrors.NewAlreadyExists(rbacv1.Resource("rolebindings"), key)
				}
				st.roleBindings[key] = binding
				return nil
			},
			updateRoleBinding: func(ctx context.Context, _ client.Client, binding *rbacv1.RoleBinding) error {
				key := types.NamespacedName{Namespace: binding.Namespace, Name: binding.Name}.String()
				_, ok := st.roleBindings[key]
				if !ok {
					return apierrors.NewNotFound(rbacv1.Resource("rolebindings"), key)
				}
				st.roleBindings[key] = binding
				return nil
			},
			getNamespace: func(ctx context.Context, _ cache.Cache, name string) (*corev1.Namespace, error) {
				namespace, ok := namespaces[name]
				if !ok {
					return nil, apierrors.NewNotFound(corev1.Resource("namespaces"), name)
				}
				return namespace, nil
			},
		}
	}

	export := &kubebindv1alpha2.APIServiceExport{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-export",
			Namespace: "kube-binder-abcd1234",
		},
		Spec: kubebindv1alpha2.APIServiceExportSpec{
			Resources: []kubebindv1alpha2.APIServiceExportResource{
				{
					GroupResource: kubebindv1alpha2.GroupResource{
						Group:    "wildwest.dev",
						Resource: "cowboys",
					},
					Versions: []string{"v1alpha1"},
				},
				{
					GroupResource: kubebindv1alpha2.GroupResource{
						Group:    "wildwest.dev",
						Resource: "cowgirls",
					},
					Versions: []string{"v1alpha1"},
				},
			},
			PermissionClaims: []kubebindv1alpha2.PermissionClaim{
				{
					GroupResource: kubebindv1alpha2.GroupResource{
						Group:    "",
						Resource: "secrets",
					},
				},
				{
					GroupResource: kubebindv1alpha2.GroupResource{
						Group:    "",
						Resource: "configmaps",
					},
				},
			},
		},
	}

	tests := map[string]struct {
		scope                kubebindv1alpha2.InformerScope
		namespaces           map[string]*corev1.Namespace
		apiServiceNamespaces map[types.NamespacedName]*kubebindv1alpha2.APIServiceNamespace
		st                   state

		expectedState state
		expectedErr   error
	}{
		"cluster scoped with no pre-existing RBACs": {
			scope: kubebindv1alpha2.ClusterScope,
			namespaces: map[string]*corev1.Namespace{
				"kube-binder-abcd1234": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "kube-binder-abcd1234",
						UID:  "uid-123",
					},
				},
			},
			apiServiceNamespaces: map[types.NamespacedName]*kubebindv1alpha2.APIServiceNamespace{
				{Namespace: "kube-binder-abcd1234", Name: "consumer-ns-1"}: {
					ObjectMeta: metav1.ObjectMeta{
						Name:      "consumer-ns-1",
						Namespace: "kube-binder-abcd1234",
					},
					Status: kubebindv1alpha2.APIServiceNamespaceStatus{
						Namespace: "kube-binder-abcd1234-consumer-ns-1",
					},
				},
			},
			st: state{},
			expectedState: state{
				clusterRoles: map[string]*rbacv1.ClusterRole{
					"kube-binder-exports": {
						ObjectMeta: metav1.ObjectMeta{
							Name: "kube-binder-exports",
						},
						AggregationRule: &rbacv1.AggregationRule{
							ClusterRoleSelectors: []metav1.LabelSelector{
								{
									MatchLabels: map[string]string{
										"rbac.kube-bind.io/aggregate-to-parent": "true",
									},
								},
							},
						},
					},
					"kube-binder-resources-kube-binder-abcd1234-my-export": {
						ObjectMeta: metav1.ObjectMeta{
							Name: "kube-binder-resources-kube-binder-abcd1234-my-export",
							Labels: map[string]string{
								"rbac.kube-bind.io/aggregate-to-parent": "true",
							},
							OwnerReferences: []metav1.OwnerReference{
								{
									APIVersion: "v1",
									Kind:       "Namespace",
									Name:       "kube-binder-abcd1234",
									UID:        "uid-123",
								},
							},
						},
						Rules: []rbacv1.PolicyRule{
							{
								APIGroups: []string{"wildwest.dev"},
								Resources: []string{"cowboys"},
								Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
							},
							{
								APIGroups: []string{"wildwest.dev"},
								Resources: []string{"cowgirls"},
								Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
							},
						},
					},
					"kube-binder-claims-kube-binder-abcd1234-my-export": {
						ObjectMeta: metav1.ObjectMeta{
							Name: "kube-binder-claims-kube-binder-abcd1234-my-export",
							Labels: map[string]string{
								"rbac.kube-bind.io/aggregate-to-parent": "true",
							},
							OwnerReferences: []metav1.OwnerReference{
								{
									APIVersion: "v1",
									Kind:       "Namespace",
									Name:       "kube-binder-abcd1234",
									UID:        "uid-123",
								},
							},
						},
						Rules: []rbacv1.PolicyRule{
							{
								APIGroups: []string{""},
								Resources: []string{"configmaps"},
								Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
							},
							{
								APIGroups: []string{""},
								Resources: []string{"secrets"},
								Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
							},
						},
					},
				},
				clusterRoleBindings: map[string]*rbacv1.ClusterRoleBinding{
					"kube-binder-exports": {
						ObjectMeta: metav1.ObjectMeta{
							Name: "kube-binder-exports",
						},
						RoleRef: rbacv1.RoleRef{
							APIGroup: rbacv1.SchemeGroupVersion.Group,
							Kind:     "ClusterRole",
							Name:     "kube-binder-exports",
						},
						Subjects: []rbacv1.Subject{
							{
								Kind:      "ServiceAccount",
								Namespace: "kube-binder-abcd1234",
								Name:      "kube-binder",
							},
						},
					},
				},
			},
		},
		"cluster scoped with wrong pre-existing RBACs": {
			scope: kubebindv1alpha2.ClusterScope,
			namespaces: map[string]*corev1.Namespace{
				"kube-binder-abcd1234": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "kube-binder-abcd1234",
						UID:  "uid-123",
					},
				},
			},
			apiServiceNamespaces: map[types.NamespacedName]*kubebindv1alpha2.APIServiceNamespace{
				{Namespace: "kube-binder-abcd1234", Name: "consumer-ns-1"}: {
					ObjectMeta: metav1.ObjectMeta{
						Name:      "consumer-ns-1",
						Namespace: "kube-binder-abcd1234",
					},
					Status: kubebindv1alpha2.APIServiceNamespaceStatus{
						Namespace: "kube-binder-abcd1234-consumer-ns-1",
					},
				},
			},
			st: state{
				clusterRoles: map[string]*rbacv1.ClusterRole{
					"kube-binder-exports": {
						ObjectMeta: metav1.ObjectMeta{
							Name: "kube-binder-exports",
						},
					},
					"kube-binder-resources-kube-binder-abcd1234-my-export": {
						ObjectMeta: metav1.ObjectMeta{
							Name: "kube-binder-resources-kube-binder-abcd1234-my-export",
						},
					},
					"kube-binder-claims-kube-binder-abcd1234-my-export": {
						ObjectMeta: metav1.ObjectMeta{
							Name: "kube-binder-claims-kube-binder-abcd1234-my-export",
						},
					},
				},
				clusterRoleBindings: map[string]*rbacv1.ClusterRoleBinding{
					"kube-binder-exports": {
						ObjectMeta: metav1.ObjectMeta{
							Name: "kube-binder-exports",
						},
						RoleRef: rbacv1.RoleRef{
							APIGroup: rbacv1.SchemeGroupVersion.Group,
							Kind:     "ClusterRole",
							Name:     "kube-binder-exports",
						},
					},
				},
			},
			expectedState: state{
				clusterRoles: map[string]*rbacv1.ClusterRole{
					"kube-binder-exports": {
						ObjectMeta: metav1.ObjectMeta{
							Name: "kube-binder-exports",
						},
						AggregationRule: &rbacv1.AggregationRule{
							ClusterRoleSelectors: []metav1.LabelSelector{
								{
									MatchLabels: map[string]string{
										"rbac.kube-bind.io/aggregate-to-parent": "true",
									},
								},
							},
						},
					},
					"kube-binder-resources-kube-binder-abcd1234-my-export": {
						ObjectMeta: metav1.ObjectMeta{
							Name: "kube-binder-resources-kube-binder-abcd1234-my-export",
							Labels: map[string]string{
								"rbac.kube-bind.io/aggregate-to-parent": "true",
							},
							OwnerReferences: []metav1.OwnerReference{
								{
									APIVersion: "v1",
									Kind:       "Namespace",
									Name:       "kube-binder-abcd1234",
									UID:        "uid-123",
								},
							},
						},
						Rules: []rbacv1.PolicyRule{
							{
								APIGroups: []string{"wildwest.dev"},
								Resources: []string{"cowboys"},
								Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
							},
							{
								APIGroups: []string{"wildwest.dev"},
								Resources: []string{"cowgirls"},
								Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
							},
						},
					},
					"kube-binder-claims-kube-binder-abcd1234-my-export": {
						ObjectMeta: metav1.ObjectMeta{
							Name: "kube-binder-claims-kube-binder-abcd1234-my-export",
							Labels: map[string]string{
								"rbac.kube-bind.io/aggregate-to-parent": "true",
							},
							OwnerReferences: []metav1.OwnerReference{
								{
									APIVersion: "v1",
									Kind:       "Namespace",
									Name:       "kube-binder-abcd1234",
									UID:        "uid-123",
								},
							},
						},
						Rules: []rbacv1.PolicyRule{
							{
								APIGroups: []string{""},
								Resources: []string{"configmaps"},
								Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
							},
							{
								APIGroups: []string{""},
								Resources: []string{"secrets"},
								Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
							},
						},
					},
				},
				clusterRoleBindings: map[string]*rbacv1.ClusterRoleBinding{
					"kube-binder-exports": {
						ObjectMeta: metav1.ObjectMeta{
							Name: "kube-binder-exports",
						},
						RoleRef: rbacv1.RoleRef{
							APIGroup: rbacv1.SchemeGroupVersion.Group,
							Kind:     "ClusterRole",
							Name:     "kube-binder-exports",
						},
						Subjects: []rbacv1.Subject{
							{
								Kind:      "ServiceAccount",
								Namespace: "kube-binder-abcd1234",
								Name:      "kube-binder",
							},
						},
					},
				},
			},
		},
		"namespaced scoped with no pre-existing RBACs": {
			scope: kubebindv1alpha2.NamespacedScope,
			namespaces: map[string]*corev1.Namespace{
				"kube-binder-abcd1234": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "kube-binder-abcd1234",
						UID:  "uid-123",
					},
				},
			},
			apiServiceNamespaces: map[types.NamespacedName]*kubebindv1alpha2.APIServiceNamespace{
				{Namespace: "kube-binder-abcd1234", Name: "consumer-ns-1"}: {
					ObjectMeta: metav1.ObjectMeta{
						Name:      "consumer-ns-1",
						Namespace: "kube-binder-abcd1234",
					},
					Status: kubebindv1alpha2.APIServiceNamespaceStatus{
						Namespace: "kube-binder-abcd1234-consumer-ns-1",
					},
				},
			},
			st: state{},
			expectedState: state{
				clusterRoles: map[string]*rbacv1.ClusterRole{
					"kube-binder-exports": {
						ObjectMeta: metav1.ObjectMeta{
							Name: "kube-binder-exports",
						},
						AggregationRule: &rbacv1.AggregationRule{
							ClusterRoleSelectors: []metav1.LabelSelector{
								{
									MatchLabels: map[string]string{
										"rbac.kube-bind.io/aggregate-to-parent": "true",
									},
								},
							},
						},
					},
					"kube-binder-resources-kube-binder-abcd1234-my-export": {
						ObjectMeta: metav1.ObjectMeta{
							Name: "kube-binder-resources-kube-binder-abcd1234-my-export",
							Labels: map[string]string{
								"rbac.kube-bind.io/aggregate-to-parent": "true",
							},
							OwnerReferences: []metav1.OwnerReference{
								{
									APIVersion: "v1",
									Kind:       "Namespace",
									Name:       "kube-binder-abcd1234",
									UID:        "uid-123",
								},
							},
						},
						Rules: []rbacv1.PolicyRule{
							{
								APIGroups: []string{"wildwest.dev"},
								Resources: []string{"cowboys"},
								Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
							},
							{
								APIGroups: []string{"wildwest.dev"},
								Resources: []string{"cowgirls"},
								Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
							},
						},
					},
					"kube-binder-claims-kube-binder-abcd1234-my-export": {
						ObjectMeta: metav1.ObjectMeta{
							Name: "kube-binder-claims-kube-binder-abcd1234-my-export",
							Labels: map[string]string{
								"rbac.kube-bind.io/aggregate-to-parent": "true",
							},
							OwnerReferences: []metav1.OwnerReference{
								{
									APIVersion: "v1",
									Kind:       "Namespace",
									Name:       "kube-binder-abcd1234",
									UID:        "uid-123",
								},
							},
						},
						Rules: []rbacv1.PolicyRule{},
					},
				},
				clusterRoleBindings: map[string]*rbacv1.ClusterRoleBinding{
					"kube-binder-exports": {
						ObjectMeta: metav1.ObjectMeta{
							Name: "kube-binder-exports",
						},
						RoleRef: rbacv1.RoleRef{
							APIGroup: rbacv1.SchemeGroupVersion.Group,
							Kind:     "ClusterRole",
							Name:     "kube-binder-exports",
						},
						Subjects: []rbacv1.Subject{
							{
								Kind:      "ServiceAccount",
								Namespace: "kube-binder-abcd1234",
								Name:      "kube-binder",
							},
						},
					},
				},
				roles: map[string]*rbacv1.Role{
					"kube-binder-abcd1234-consumer-ns-1/kube-binder-claims-my-export": {
						ObjectMeta: metav1.ObjectMeta{
							Name:      "kube-binder-claims-my-export",
							Namespace: "kube-binder-abcd1234-consumer-ns-1",
						},
						Rules: []rbacv1.PolicyRule{
							{
								APIGroups: []string{""},
								Resources: []string{"configmaps"},
								Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
							},
							{
								APIGroups: []string{""},
								Resources: []string{"secrets"},
								Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
							},
						},
					},
				},
				roleBindings: map[string]*rbacv1.RoleBinding{
					"kube-binder-abcd1234-consumer-ns-1/kube-binder-claims-my-export": {
						ObjectMeta: metav1.ObjectMeta{
							Name:      "kube-binder-claims-my-export",
							Namespace: "kube-binder-abcd1234-consumer-ns-1",
						},
						RoleRef: rbacv1.RoleRef{
							APIGroup: rbacv1.SchemeGroupVersion.Group,
							Kind:     "Role",
							Name:     "kube-binder-claims-my-export",
						},
						Subjects: []rbacv1.Subject{
							{
								Kind:      "ServiceAccount",
								Namespace: "kube-binder-abcd1234",
								Name:      "kube-binder",
							},
						},
					},
				},
			},
		},
		"namespaced scoped with wrong pre-existing RBACs": {
			scope: kubebindv1alpha2.NamespacedScope,
			namespaces: map[string]*corev1.Namespace{
				"kube-binder-abcd1234": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "kube-binder-abcd1234",
						UID:  "uid-123",
					},
				},
			},
			apiServiceNamespaces: map[types.NamespacedName]*kubebindv1alpha2.APIServiceNamespace{
				{Namespace: "kube-binder-abcd1234", Name: "consumer-ns-1"}: {
					ObjectMeta: metav1.ObjectMeta{
						Name:      "consumer-ns-1",
						Namespace: "kube-binder-abcd1234",
					},
					Status: kubebindv1alpha2.APIServiceNamespaceStatus{
						Namespace: "kube-binder-abcd1234-consumer-ns-1",
					},
				},
			},
			st: state{
				clusterRoles: map[string]*rbacv1.ClusterRole{
					"kube-binder-exports": {
						ObjectMeta: metav1.ObjectMeta{
							Name: "kube-binder-exports",
						},
					},
					"kube-binder-resources-kube-binder-abcd1234-my-export": {
						ObjectMeta: metav1.ObjectMeta{
							Name: "kube-binder-resources-kube-binder-abcd1234-my-export",
						},
					},
					"kube-binder-claims-kube-binder-abcd1234-my-export": {
						ObjectMeta: metav1.ObjectMeta{
							Name: "kube-binder-claims-kube-binder-abcd1234-my-export",
						},
						Rules: []rbacv1.PolicyRule{},
					},
				},
				clusterRoleBindings: map[string]*rbacv1.ClusterRoleBinding{
					"kube-binder-exports": {
						ObjectMeta: metav1.ObjectMeta{
							Name: "kube-binder-exports",
						},
						RoleRef: rbacv1.RoleRef{
							APIGroup: rbacv1.SchemeGroupVersion.Group,
							Kind:     "ClusterRole",
							Name:     "kube-binder-exports",
						},
					},
				},
				roles: map[string]*rbacv1.Role{
					"kube-binder-abcd1234-consumer-ns-1/kube-binder-claims-my-export": {
						ObjectMeta: metav1.ObjectMeta{
							Name:      "kube-binder-claims-my-export",
							Namespace: "kube-binder-abcd1234-consumer-ns-1",
						},
					},
				},
				roleBindings: map[string]*rbacv1.RoleBinding{
					"kube-binder-abcd1234-consumer-ns-1/kube-binder-claims-my-export": {
						ObjectMeta: metav1.ObjectMeta{
							Name:      "kube-binder-claims-my-export",
							Namespace: "kube-binder-abcd1234-consumer-ns-1",
						},
						RoleRef: rbacv1.RoleRef{
							APIGroup: rbacv1.SchemeGroupVersion.Group,
							Kind:     "Role",
							Name:     "kube-binder-claims-my-export",
						},
					},
				},
			},
			expectedState: state{
				clusterRoles: map[string]*rbacv1.ClusterRole{
					"kube-binder-exports": {
						ObjectMeta: metav1.ObjectMeta{
							Name: "kube-binder-exports",
						},
						AggregationRule: &rbacv1.AggregationRule{
							ClusterRoleSelectors: []metav1.LabelSelector{
								{
									MatchLabels: map[string]string{
										"rbac.kube-bind.io/aggregate-to-parent": "true",
									},
								},
							},
						},
					},
					"kube-binder-resources-kube-binder-abcd1234-my-export": {
						ObjectMeta: metav1.ObjectMeta{
							Name: "kube-binder-resources-kube-binder-abcd1234-my-export",
							Labels: map[string]string{
								"rbac.kube-bind.io/aggregate-to-parent": "true",
							},
							OwnerReferences: []metav1.OwnerReference{
								{
									APIVersion: "v1",
									Kind:       "Namespace",
									Name:       "kube-binder-abcd1234",
									UID:        "uid-123",
								},
							},
						},
						Rules: []rbacv1.PolicyRule{
							{
								APIGroups: []string{"wildwest.dev"},
								Resources: []string{"cowboys"},
								Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
							},
							{
								APIGroups: []string{"wildwest.dev"},
								Resources: []string{"cowgirls"},
								Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
							},
						},
					},
					"kube-binder-claims-kube-binder-abcd1234-my-export": {
						ObjectMeta: metav1.ObjectMeta{
							Name: "kube-binder-claims-kube-binder-abcd1234-my-export",
							Labels: map[string]string{
								"rbac.kube-bind.io/aggregate-to-parent": "true",
							},
							OwnerReferences: []metav1.OwnerReference{
								{
									APIVersion: "v1",
									Kind:       "Namespace",
									Name:       "kube-binder-abcd1234",
									UID:        "uid-123",
								},
							},
						},
						Rules: []rbacv1.PolicyRule{},
					},
				},
				clusterRoleBindings: map[string]*rbacv1.ClusterRoleBinding{
					"kube-binder-exports": {
						ObjectMeta: metav1.ObjectMeta{
							Name: "kube-binder-exports",
						},
						RoleRef: rbacv1.RoleRef{
							APIGroup: rbacv1.SchemeGroupVersion.Group,
							Kind:     "ClusterRole",
							Name:     "kube-binder-exports",
						},
						Subjects: []rbacv1.Subject{
							{
								Kind:      "ServiceAccount",
								Namespace: "kube-binder-abcd1234",
								Name:      "kube-binder",
							},
						},
					},
				},
				roles: map[string]*rbacv1.Role{
					"kube-binder-abcd1234-consumer-ns-1/kube-binder-claims-my-export": {
						ObjectMeta: metav1.ObjectMeta{
							Name:      "kube-binder-claims-my-export",
							Namespace: "kube-binder-abcd1234-consumer-ns-1",
						},
						Rules: []rbacv1.PolicyRule{
							{
								APIGroups: []string{""},
								Resources: []string{"configmaps"},
								Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
							},
							{
								APIGroups: []string{""},
								Resources: []string{"secrets"},
								Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
							},
						},
					},
				},
				roleBindings: map[string]*rbacv1.RoleBinding{
					"kube-binder-abcd1234-consumer-ns-1/kube-binder-claims-my-export": {
						ObjectMeta: metav1.ObjectMeta{
							Name:      "kube-binder-claims-my-export",
							Namespace: "kube-binder-abcd1234-consumer-ns-1",
						},
						RoleRef: rbacv1.RoleRef{
							APIGroup: rbacv1.SchemeGroupVersion.Group,
							Kind:     "Role",
							Name:     "kube-binder-claims-my-export",
						},
						Subjects: []rbacv1.Subject{
							{
								Kind:      "ServiceAccount",
								Namespace: "kube-binder-abcd1234",
								Name:      "kube-binder",
							},
						},
					},
				},
			},
		},
	}

	for tname, tt := range tests {
		t.Run(tname, func(t *testing.T) {
			r := newReconcilerWithState(tt.scope, tt.namespaces, tt.apiServiceNamespaces, &tt.st)

			normalizeState(&tt.st)
			normalizeState(&tt.expectedState)

			err := r.reconcile(context.Background(), nil, nil, export)

			if tt.expectedErr != nil {
				require.EqualError(t, err, tt.expectedErr.Error(), "reconcile should have failed")
			} else {
				require.NoError(t, err, "reconcile should have succeeded")
			}

			compareMap(t, "ClusterRole", tt.expectedState.clusterRoles, tt.st.clusterRoles)
			compareMap(t, "ClusterRoleBinding", tt.expectedState.clusterRoleBindings, tt.st.clusterRoleBindings)
			compareMap(t, "Role", tt.expectedState.roles, tt.st.roles)
			compareMap(t, "RoleBinding", tt.expectedState.roleBindings, tt.st.roleBindings)
		})
	}
}

func compareMap[V any](t *testing.T, kind string, a, b map[string]V) {
	require.Equal(t, slices.Sorted(maps.Keys(a)), slices.Sorted(maps.Keys(b)), "got unexpected entries after %s reconciliation", kind)
	for k := range a {
		require.Equal(t, a[k], b[k], "got unexpected content in %s %s", k, kind)
	}
}
