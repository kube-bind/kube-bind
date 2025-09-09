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

package serviceexport

import (
	"context"
	"fmt"

	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kuberesources "github.com/kube-bind/kube-bind/backend/kubernetes/resources"
	kubebindv1alpha2 "github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2"
	"github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2/helpers"
	"github.com/kube-bind/kube-bind/sdk/apis/third_party/conditions/util/conditions"
)

type reconciler struct {
	scope kubebindv1alpha2.InformerScope
}

func (r *reconciler) reconcile(ctx context.Context, cache cache.Cache, client client.Client, export *kubebindv1alpha2.APIServiceExport) error {
	var errs []error
	log := klog.FromContext(ctx)

	if specChanged, err := r.ensureSchema(ctx, cache, export); err != nil {
		errs = append(errs, err)
	} else if specChanged {
		// TODO(mjudeikis): Implement schema lifecycle. Based on how system is configured, we need to either force crd/schema updates
		// or not. This will require a more in-depth analysis of the current system and its requirements. Idea is that we need to watch GVR of schema
		// source and based on some policy setting update boundschemas or not. And propagate them to consumer.
		// https://github.com/kube-bind/kube-bind/issues/301
		log.Info("APIServiceExport schema change detected. Update not implemented.")
	}

	err := r.ensurePermissionClaimsPermissions(ctx, cache, client, export)
	if err != nil {
		errs = append(errs, err)
	}

	return utilerrors.NewAggregate(errs)
}

func (r *reconciler) ensureSchema(ctx context.Context, cache cache.Cache, export *kubebindv1alpha2.APIServiceExport) (specChanged bool, err error) {
	logger := klog.FromContext(ctx)
	schemas := make([]*kubebindv1alpha2.BoundSchema, 0, len(export.Spec.Resources))

	for _, res := range export.Spec.Resources {
		name := res.Resource + "." + res.Group
		schema, err := r.getBoundSchema(ctx, cache, name)
		if err != nil {
			if errors.IsNotFound(err) {
				continue
			}
			return false, err
		}

		schemas = append(schemas, schema)
	}

	hash := helpers.BoundSchemasSpecHash(schemas)

	if export.Annotations[kubebindv1alpha2.SourceSpecHashAnnotationKey] != hash {
		// both exist, update APIServiceExport
		logger.V(1).Info("Updating APIServiceExport. Hash mismatch", "hash", hash, "expected", export.Annotations[kubebindv1alpha2.SourceSpecHashAnnotationKey])
		if export.Annotations == nil {
			export.Annotations = map[string]string{}
		}
		export.Annotations[kubebindv1alpha2.SourceSpecHashAnnotationKey] = hash
		return true, nil
	}

	conditions.MarkTrue(export, kubebindv1alpha2.APIServiceExportConditionProviderInSync)

	return false, nil
}

func (r *reconciler) ensurePermissionClaimsPermissions(ctx context.Context, cache cache.Cache, client client.Client, export *kubebindv1alpha2.APIServiceExport) error {
	// Ensure that the permission claims for the APIServiceExport are correctly set up
	namespace := export.Namespace               // This is where cluster service account lives
	name := "kube-kinder-export-" + export.Name // unique name for the service export related permissions

	owner := metav1.NewControllerRef(export, kubebindv1alpha2.SchemeGroupVersion.WithKind(kubebindv1alpha2.KindAPIServiceExport))

	permissions := []rbacv1.PolicyRule{}
	for _, claim := range export.Spec.PermissionClaims {
		resourceNames := []string{}
		for _, name := range claim.Selector.ResourceNames {
			resourceNames = append(resourceNames, name.Name)
		}

		permissions = append(permissions, rbacv1.PolicyRule{
			APIGroups:     []string{claim.Group},
			Resources:     []string{claim.Resource},
			Verbs:         claim.Verbs,
			ResourceNames: resourceNames,
		})
	}

	// For clusterscope isolation we create ClusterRole and for namespace - namespace.
	// There is a risk that user using cluster level isolation would get more permissions than expected,
	// but this is expected.
	if r.scope == kubebindv1alpha2.ClusterScope {
		role, err := r.getPermissionClaimsClusterRole(ctx, cache, name)
		if err != nil && !errors.IsNotFound(err) {
			return fmt.Errorf("failed to get Role %s: %w", name, err)
		}
		if role == nil {
			role = &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name:            name,
					Namespace:       namespace,
					OwnerReferences: []metav1.OwnerReference{*owner},
				},
				Rules: permissions,
			}
			// Create new ClusterRole
			if err := client.Create(ctx, role); err != nil {
				return fmt.Errorf("failed to create ClusterRole %s: %w", name, err)
			}
		} else {
			role.Rules = permissions
			if err := client.Update(ctx, role); err != nil {
				return fmt.Errorf("failed to update ClusterRole %s: %w", name, err)
			}
		}
	} else {
		role, err := r.getPermissionClaimsRole(ctx, cache, namespace, name)
		if err != nil && !errors.IsNotFound(err) {
			return fmt.Errorf("failed to get Role %s: %w", name, err)
		}
		if role == nil {
			role = &rbacv1.Role{
				ObjectMeta: metav1.ObjectMeta{
					Name:            name,
					Namespace:       namespace,
					OwnerReferences: []metav1.OwnerReference{*owner},
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
	}

	// Ensure {Cluster}RoleBinding
	if r.scope == kubebindv1alpha2.ClusterScope {
		roleBinding, err := r.getPermissionClaimsClusterRoleBinding(ctx, cache, name)
		if err != nil && !errors.IsNotFound(err) {
			return fmt.Errorf("failed to get ClusterRoleBinding %s: %w", name, err)
		}
		if roleBinding == nil {
			roleBinding = &rbacv1.ClusterRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:            name,
					OwnerReferences: []metav1.OwnerReference{*owner},
				},
				RoleRef: rbacv1.RoleRef{
					APIGroup: rbacv1.SchemeGroupVersion.Group,
					Kind:     "ClusterRole",
					Name:     name,
				},
				Subjects: []rbacv1.Subject{
					{
						APIGroup:  "",
						Kind:      "ServiceAccount",
						Name:      kuberesources.ServiceAccountName,
						Namespace: namespace,
					},
				},
			}
			// Create new ClusterRoleBinding
			if err := client.Create(ctx, roleBinding); err != nil {
				return fmt.Errorf("failed to create ClusterRoleBinding %s: %w", name, err)
			}
		}
		// No update implemented.
	} else {
		roleBinding, err := r.getPermissionClaimsRoleBinding(ctx, cache, namespace, name)
		if err != nil && !errors.IsNotFound(err) {
			return fmt.Errorf("failed to get RoleBinding %s: %w", name, err)
		}
		if roleBinding == nil {
			roleBinding = &rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:            name,
					Namespace:       namespace,
					OwnerReferences: []metav1.OwnerReference{*owner},
				},
				RoleRef: rbacv1.RoleRef{
					APIGroup: rbacv1.SchemeGroupVersion.Group,
					Kind:     "Role",
					Name:     name,
				},
				Subjects: []rbacv1.Subject{
					{
						APIGroup:  "",
						Kind:      "ServiceAccount",
						Name:      kuberesources.ServiceAccountName,
						Namespace: namespace,
					},
				},
			}
			// Create new RoleBinding
			if err := client.Create(ctx, roleBinding); err != nil {
				return fmt.Errorf("failed to create RoleBinding %s: %w", name, err)
			}
		}
		// No update implemented.
	}

	return nil
}

func (r *reconciler) getPermissionClaimsClusterRole(ctx context.Context, cache cache.Cache, name string) (*rbacv1.ClusterRole, error) {
	var role rbacv1.ClusterRole
	key := types.NamespacedName{Name: name}
	if err := cache.Get(ctx, key, &role); err != nil {
		return nil, err
	}
	return &role, nil
}

func (r *reconciler) getPermissionClaimsRole(ctx context.Context, cache cache.Cache, namespace, name string) (*rbacv1.Role, error) {
	var role rbacv1.Role
	key := types.NamespacedName{Namespace: namespace, Name: name}
	if err := cache.Get(ctx, key, &role); err != nil {
		return nil, err
	}
	return &role, nil
}

func (r *reconciler) getBoundSchema(ctx context.Context, cache cache.Cache, name string) (*kubebindv1alpha2.BoundSchema, error) {
	var schema kubebindv1alpha2.BoundSchema
	key := types.NamespacedName{Name: name}
	if err := cache.Get(ctx, key, &schema); err != nil {
		return nil, err
	}
	return &schema, nil
}

func (r *reconciler) getPermissionClaimsClusterRoleBinding(ctx context.Context, cache cache.Cache, name string) (*rbacv1.ClusterRoleBinding, error) {
	var roleBinding rbacv1.ClusterRoleBinding
	key := types.NamespacedName{Name: name}
	if err := cache.Get(ctx, key, &roleBinding); err != nil {
		return nil, err
	}
	return &roleBinding, nil
}

func (r *reconciler) getPermissionClaimsRoleBinding(ctx context.Context, cache cache.Cache, namespace, name string) (*rbacv1.RoleBinding, error) {
	var roleBinding rbacv1.RoleBinding
	key := types.NamespacedName{Name: name, Namespace: namespace}
	if err := cache.Get(ctx, key, &roleBinding); err != nil {
		return nil, err
	}
	return &roleBinding, nil
}
