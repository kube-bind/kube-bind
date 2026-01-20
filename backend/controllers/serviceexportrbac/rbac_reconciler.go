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
	"fmt"
	"maps"
	"reflect"
	"slices"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	kuberesources "github.com/kube-bind/kube-bind/backend/kubernetes/resources"
	kubebindv1alpha2 "github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2"
)

type reconciler struct {
	scope kubebindv1alpha2.InformerScope

	listServiceNamespaces func(ctx context.Context, cache cache.Cache, namespace string) ([]*kubebindv1alpha2.APIServiceNamespace, error)
	getNamespace          func(ctx context.Context, cache cache.Cache, name string) (*corev1.Namespace, error)

	getClusterRole    func(ctx context.Context, cache cache.Cache, name string) (*rbacv1.ClusterRole, error)
	createClusterRole func(ctx context.Context, client client.Client, clusterRole *rbacv1.ClusterRole) error
	updateClusterRole func(ctx context.Context, client client.Client, clusterRole *rbacv1.ClusterRole) error

	getClusterRoleBinding    func(ctx context.Context, cache cache.Cache, name string) (*rbacv1.ClusterRoleBinding, error)
	createClusterRoleBinding func(ctx context.Context, client client.Client, clusterRoleBinding *rbacv1.ClusterRoleBinding) error
	updateClusterRoleBinding func(ctx context.Context, client client.Client, clusterRoleBinding *rbacv1.ClusterRoleBinding) error

	getRole    func(ctx context.Context, cache cache.Cache, namespace, name string) (*rbacv1.Role, error)
	createRole func(ctx context.Context, client client.Client, role *rbacv1.Role) error
	updateRole func(ctx context.Context, client client.Client, role *rbacv1.Role) error

	getRoleBinding    func(ctx context.Context, cache cache.Cache, namespace, name string) (*rbacv1.RoleBinding, error)
	createRoleBinding func(ctx context.Context, client client.Client, roleBinding *rbacv1.RoleBinding) error
	updateRoleBinding func(ctx context.Context, client client.Client, roleBinding *rbacv1.RoleBinding) error
}

const (
	rbacAggregateLabelKey = "rbac.kube-bind.io/aggregate-to-parent"
)

func (r *reconciler) reconcile(ctx context.Context, client client.Client, cache cache.Cache, export *kubebindv1alpha2.APIServiceExport) error {
	logger := log.FromContext(ctx)

	var errs []error

	// Retrieve policy rules for exported and claimed resources.

	// buildPolicyRules transforms the input groupResourceSet
	// into a slice of PolicyRules with deterministic ordering.
	buildPolicyRules := func(verbs []string, groupResourceSet map[string]metav1.GroupResource) []rbacv1.PolicyRule {
		rules := make([]rbacv1.PolicyRule, 0, len(groupResourceSet))
		for _, key := range slices.Sorted(maps.Keys(groupResourceSet)) {
			gr := groupResourceSet[key]
			rules = append(rules, rbacv1.PolicyRule{
				APIGroups: []string{gr.Group},
				Resources: []string{gr.Resource},
				Verbs:     verbs,
			})
		}
		return rules
	}

	resourceGroupResourceSet := make(map[string]metav1.GroupResource)
	for _, res := range export.Spec.Resources {
		resourceGroupResourceSet[res.GroupResource.String()] = metav1.GroupResource(res.GroupResource)
	}
	resourceRules := buildPolicyRules(
		[]string{"get", "list", "watch", "create", "update", "patch", "delete"},
		resourceGroupResourceSet,
	)

	clusterScopedClaimsGroupResourceSet := make(map[string]metav1.GroupResource)
	namespaceScopedClaimsGroupResourceSet := make(map[string]metav1.GroupResource)
	for _, claim := range export.Spec.PermissionClaims {
		var claimScope apiextensionsv1.ResourceScope
		for _, claimableAPI := range kubebindv1alpha2.ClaimableAPIs {
			if claim.Group == claimableAPI.GroupVersionResource.Group && claim.Resource == claimableAPI.GroupVersionResource.Resource {
				claimScope = claimableAPI.ResourceScope
				break
			}
		}
		switch claimScope {
		case apiextensionsv1.ClusterScoped:
			clusterScopedClaimsGroupResourceSet[claim.GroupResource.String()] = metav1.GroupResource(claim.GroupResource)
		case apiextensionsv1.NamespaceScoped:
			namespaceScopedClaimsGroupResourceSet[claim.GroupResource.String()] = metav1.GroupResource(claim.GroupResource)
		default:
			// This claimed resource is not in our claimable APIs list.
			// There is nothing we can do about it here, just skip it.
			logger.V(2).Info("Skipping RBAC reconciliation for claimed resource because it's not a claimable API",
				"resource", claim.GroupResource.String())
			continue
		}
	}
	clusterScopedClaimRules := buildPolicyRules(
		[]string{"get", "list", "watch", "create", "update", "patch", "delete"},
		clusterScopedClaimsGroupResourceSet,
	)
	namespaceScopedClaimRules := buildPolicyRules(
		[]string{"get", "list", "watch", "create", "update", "patch", "delete"},
		namespaceScopedClaimsGroupResourceSet,
	)

	// Here we ensure RBAC objects are in place.
	//
	// Cluster-scoped RBACs:
	//
	// - kube-binder-exports ClusterRole and ClusterRoleBinding
	//     * bound to <APIServiceExport.Namespace>/kube-binder ServiceAccount
	//   Aggregating ClusterRole for all subsequent ClusterRoles we create.
	// - kube-binder-resources-<APIServiceExport.Namespace>-<APIServiceExport.Name> ClusterRole
	//   Access to all resources declared in APIServiceExport.Spec.Resources.
	//   Aggregates into kube-binder-exports ClusterRole.
	// - kube-binder-claims-<APIServiceExport.Namespace>-<APIServiceExport.Name> ClusterRole
	//   Access to all cluster-scoped claimed resources declared in APIServiceExport.Spec.PermissionClaims.
	//   Aggregates into kube-binder-exports ClusterRole.
	//
	// Namespace-scoped RBACs:
	//
	// - <APIServiceNamespace.Status.Namespace>/kube-binder-claims-<APIServiceExport.Name> Role and RoleBinding
	//     * bound to <APIServiceExport.Namespace>/kube-binder ServiceAccount
	//   Access to namespace-scoped claimed resources declared in APIServiceExport.Spec.PermissionClaims.

	// We use the owner ref below in "kube-binder-{resources,claims}-*"
	// ClusterRoles and ClusterRoleBindings for automatic cleanup.
	ns, err := r.getNamespace(ctx, cache, export.Namespace)
	if err != nil {
		return err
	}
	ownerRefs := []metav1.OwnerReference{
		{
			APIVersion: "v1",
			Kind:       "Namespace",
			Name:       ns.Name,
			Controller: ptr.To(true),
			UID:        ns.UID,
		},
	}

	// We always bind to the <APIServiceExport.Namespace>/kube-binder ServiceAccount.
	kubebinderSubject := []rbacv1.Subject{
		{
			Kind:      "ServiceAccount",
			Namespace: export.Namespace,
			Name:      kuberesources.ServiceAccountName,
		},
	}

	errs = append(errs,
		r.ensureAggregatingClusteRoleAndClusterRoleBinding(ctx, client, cache, kubebinderSubject),
		// We always use cluster-scoped RBACs for exported resources.
		r.ensureAggregatedClusterRole(ctx, client, cache,
			fmt.Sprintf("kube-binder-resources-%s-%s", export.Namespace, export.Name),
			ownerRefs,
			resourceRules,
		),
	)

	// Now onto claimed resources RBACs.

	switch r.scope {
	case kubebindv1alpha2.ClusterScope:
		errs = append(errs,
			r.ensureAggregatedClusterRole(ctx, client, cache,
				fmt.Sprintf("kube-binder-claims-%s-%s", export.Namespace, export.Name),
				ownerRefs,
				append(clusterScopedClaimRules, namespaceScopedClaimRules...),
			),
		)
	case kubebindv1alpha2.NamespacedScope:
		errs = append(errs,
			r.ensureAggregatedClusterRole(ctx, client, cache,
				fmt.Sprintf("kube-binder-claims-%s-%s", export.Namespace, export.Name),
				ownerRefs,
				clusterScopedClaimRules,
			),
		)
		serviceNamespaces, err := r.listServiceNamespaces(ctx, cache, export.Namespace)
		if err != nil {
			errs = append(errs, err)
			return utilerrors.NewAggregate(errs)
		}
		for _, serviceNamespace := range serviceNamespaces {
			if serviceNamespace.Status.Namespace == "" {
				continue
			}
			errs = append(errs,
				r.ensureRoleAndRoleBinding(ctx, client, cache,
					serviceNamespace.Status.Namespace,
					fmt.Sprintf("kube-binder-claims-%s", export.Name),
					kubebinderSubject,
					namespaceScopedClaimRules,
				),
			)
		}
	default:
		errs = append(errs, fmt.Errorf("unknown informer scope %q", r.scope)) // This should not happen!
	}

	return utilerrors.NewAggregate(errs)
}

func (r *reconciler) ensureRoleAndRoleBinding(ctx context.Context, client client.Client, cache cache.Cache, namespace, name string, subjects []rbacv1.Subject, rules []rbacv1.PolicyRule) error {
	// Ensure the Role exists.

	expectedRole := rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Rules: rules,
	}

	role, err := r.getRole(ctx, cache, namespace, name)
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	if role == nil {
		err = r.createRole(ctx, client, &expectedRole)
		if err != nil && apierrors.IsAlreadyExists(err) {
			return err
		}
		role = &expectedRole
	}
	if !reflect.DeepEqual(expectedRole.Rules, role.Rules) {
		copyRole := role.DeepCopy()
		copyRole.Rules = expectedRole.Rules
		err = r.updateRole(ctx, client, copyRole)
		if err != nil {
			return err
		}
	}

	// And also ensure its RoleBinding exists too.

	expectedRoleBinding := rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.SchemeGroupVersion.Group,
			Kind:     "Role",
			Name:     name,
		},
		Subjects: subjects,
	}

	roleBinding, err := r.getRoleBinding(ctx, cache, namespace, name)
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	if roleBinding == nil {
		err = r.createRoleBinding(ctx, client, &expectedRoleBinding)
		if err == nil {
			return nil
		}
		if !apierrors.IsAlreadyExists(err) {
			return err
		}
		roleBinding = &expectedRoleBinding
	}

	if !reflect.DeepEqual(expectedRoleBinding.Subjects, roleBinding.Subjects) {
		// We don't check .RoleRef because it's immutable.
		copyRoleBinding := roleBinding.DeepCopy()
		copyRoleBinding.Subjects = expectedRoleBinding.Subjects
		return r.updateRoleBinding(ctx, client, copyRoleBinding)
	}

	return nil
}

func (r *reconciler) ensureAggregatedClusterRole(ctx context.Context, client client.Client, cache cache.Cache, name string, owners []metav1.OwnerReference, rules []rbacv1.PolicyRule) error {
	expectedClusterRole := rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				rbacAggregateLabelKey: "true",
			},
			OwnerReferences: owners,
		},
		Rules: rules,
	}

	clusterRole, err := r.getClusterRole(ctx, cache, name)
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	if clusterRole == nil {
		err = r.createClusterRole(ctx, client, &expectedClusterRole)
		if err == nil {
			return nil
		}
		if !apierrors.IsAlreadyExists(err) {
			return err
		}
		clusterRole = &expectedClusterRole
	}

	if clusterRole.Labels == nil || clusterRole.Labels[rbacAggregateLabelKey] != "true" ||
		!reflect.DeepEqual(expectedClusterRole.OwnerReferences, clusterRole.OwnerReferences) ||
		!reflect.DeepEqual(expectedClusterRole.Rules, clusterRole.Rules) {
		// We don't check .RoleRef because it's immutable.
		copyClusterRole := clusterRole.DeepCopy()
		if copyClusterRole.Labels == nil {
			copyClusterRole.Labels = make(map[string]string)
		}
		copyClusterRole.Labels[rbacAggregateLabelKey] = "true"
		copyClusterRole.OwnerReferences = expectedClusterRole.OwnerReferences
		copyClusterRole.Rules = expectedClusterRole.Rules
		return r.updateClusterRole(ctx, client, copyClusterRole)
	}

	return nil
}

func (r *reconciler) ensureAggregatingClusteRoleAndClusterRoleBinding(ctx context.Context, client client.Client, cache cache.Cache, subjects []rbacv1.Subject) error {
	const name = "kube-binder-exports"

	// Ensure the "kube-binder-exports" ClusterRole exists.

	expectedClusterRole := rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		AggregationRule: &rbacv1.AggregationRule{
			ClusterRoleSelectors: []metav1.LabelSelector{
				{
					MatchLabels: map[string]string{
						rbacAggregateLabelKey: "true",
					},
				},
			},
		},
	}

	clusterRole, err := r.getClusterRole(ctx, cache, name)
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	if clusterRole == nil {
		err = r.createClusterRole(ctx, client, &expectedClusterRole)
		if err != nil && apierrors.IsAlreadyExists(err) {
			return err
		}
		clusterRole = &expectedClusterRole
	}

	if !reflect.DeepEqual(expectedClusterRole.AggregationRule, clusterRole.AggregationRule) {
		copyClusterRole := clusterRole.DeepCopy()
		copyClusterRole.AggregationRule = expectedClusterRole.AggregationRule
		err = r.updateClusterRole(ctx, client, copyClusterRole)
		if err != nil {
			return err
		}
	}

	// Now ensure its "kube-binder-exports" ClusterRoleBinding exists.

	expectedClusterRoleBinding := rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.SchemeGroupVersion.Group,
			Kind:     "ClusterRole",
			Name:     name,
		},
		Subjects: subjects,
	}

	clusterRoleBinding, err := r.getClusterRoleBinding(ctx, cache, name)
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	if clusterRoleBinding == nil {
		err = r.createClusterRoleBinding(ctx, client, &expectedClusterRoleBinding)
		if err == nil {
			return nil
		}
		if !apierrors.IsAlreadyExists(err) {
			return err
		}
		clusterRoleBinding = &expectedClusterRoleBinding
	}

	if !reflect.DeepEqual(expectedClusterRoleBinding.Subjects, clusterRoleBinding.Subjects) {
		// We don't check .RoleRef because it's immutable.
		copyClusterRoleBinding := clusterRoleBinding.DeepCopy()
		copyClusterRoleBinding.Subjects = expectedClusterRoleBinding.Subjects
		return r.updateClusterRoleBinding(ctx, client, copyClusterRoleBinding)
	}

	return nil
}
