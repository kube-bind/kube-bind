/*
Copyright 2026 The Kube Bind Authors.

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

// Package vwrbac is a TEMPORARY controller that bootstraps RBAC required by
// the kcp apiresourceschema virtual workspace into every consumer workspace
// that has a kube-bind APIBinding. The VW authorizer requires the caller to
// have read access to apibindings.apis.kcp.io in the consumer workspace
// before it will return APIResourceSchemas (see
// kcp/pkg/virtual/apiresourceschema/authorizer/authorizer.go). The kube-bind
// backend SA does not have that by default, so we plant a ClusterRole +
// ClusterRoleBinding once per consumer workspace.
//
// This package should be removed once kcp grants apiresourceschema VW access
// directly to the APIExport owner identity.
package vwrbac

import (
	"context"
	"fmt"
	"strings"

	apisv1alpha2 "github.com/kcp-dev/sdk/apis/apis/v1alpha2"
	authenticationv1 "k8s.io/api/authentication/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"
	mcbuilder "sigs.k8s.io/multicluster-runtime/pkg/builder"
	mcmanager "sigs.k8s.io/multicluster-runtime/pkg/manager"
	mcreconcile "sigs.k8s.io/multicluster-runtime/pkg/reconcile"
)

// kcpClusterNameExtraKey is the extra-key kcp injects on requests authenticated
// as a kcp-native ServiceAccount. Its value is the SA's home logical cluster
// name. It's the marker that lets us turn the bare SA username into the kcp
// "global SA" identity used for cross-workspace RBAC.
const kcpClusterNameExtraKey = "authentication.kcp.io/cluster-name"

const (
	controllerName = "kube-bind-kcp-vwrbac"

	// roleName is the deterministic name for the ClusterRole + ClusterRoleBinding
	// installed in every consumer workspace.
	roleName = "kube-bind-vw-apibinding-reader"
)

// VWRBACReconciler watches APIBindings across consumer workspaces and ensures
// each workspace has a ClusterRole + ClusterRoleBinding allowing the kube-bind
// backend identity to read apibindings.apis.kcp.io. This is what unlocks the
// kcp apiresourceschema VW for the backend.
type VWRBACReconciler struct {
	manager mcmanager.Manager
	opts    controller.TypedOptions[mcreconcile.Request]
	subject rbacv1.Subject
}

// New returns a new VWRBACReconciler. It performs a SelfSubjectReview against
// baseConfig to discover the identity the backend will use against consumer
// workspaces, so the bootstrapped RBAC binds the right user.
func New(
	ctx context.Context,
	mgr mcmanager.Manager,
	opts controller.TypedOptions[mcreconcile.Request],
	baseConfig *rest.Config,
) (*VWRBACReconciler, error) {
	subject, err := discoverSubject(ctx, baseConfig)
	if err != nil {
		return nil, fmt.Errorf("vwrbac: failed to discover backend identity: %w", err)
	}

	log.FromContext(ctx).Info("vwrbac: bootstrapping consumer-workspace RBAC for identity",
		"kind", subject.Kind, "name", subject.Name)

	return &VWRBACReconciler{
		manager: mgr,
		opts:    opts,
		subject: subject,
	}, nil
}

// discoverSubject calls SelfSubjectReview against the supplied config and maps
// the resulting identity into the RBAC subject that consumer-workspace
// ClusterRoleBindings must use.
//
// Two cases:
//
//  1. kcp-native ServiceAccount (the recommended setup): the JWT carries the
//     SA's home logical cluster, which kcp surfaces as the
//     "authentication.kcp.io/cluster-name" extra. Cross-workspace RBAC binds
//     it via the "global SA" identity:
//
//     system:kcp:serviceaccount:<sa-home-cluster>:<namespace>:<sa-name>
//
//     bound with Kind: User. See kcp e2e
//     test/e2e/virtual/apiresourceschema/virtualworkspace_test.go.
//
//  2. Foreign identity (e.g. a local Kubernetes SA presented via OIDC, or any
//     other User): use the bare username as Kind: User. kcp will not resolve
//     it as a ServiceAccount object — `Kind: ServiceAccount` would never
//     match.
func discoverSubject(ctx context.Context, cfg *rest.Config) (rbacv1.Subject, error) {
	kc, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return rbacv1.Subject{}, err
	}
	rev, err := kc.AuthenticationV1().SelfSubjectReviews().Create(
		ctx, &authenticationv1.SelfSubjectReview{}, metav1.CreateOptions{})
	if err != nil {
		return rbacv1.Subject{}, fmt.Errorf("SelfSubjectReview: %w", err)
	}

	user := rev.Status.UserInfo.Username
	if user == "" {
		return rbacv1.Subject{}, fmt.Errorf("SelfSubjectReview returned empty username")
	}

	name := user
	const saPrefix = "system:serviceaccount:"
	if clusters := rev.Status.UserInfo.Extra[kcpClusterNameExtraKey]; len(clusters) > 0 && strings.HasPrefix(user, saPrefix) {
		nsName := strings.TrimPrefix(user, saPrefix) // "<ns>:<sa>"
		name = fmt.Sprintf("system:kcp:serviceaccount:%s:%s", clusters[0], nsName)
	}

	return rbacv1.Subject{
		Kind:     rbacv1.UserKind,
		APIGroup: rbacv1.GroupName,
		Name:     name,
	}, nil
}

// Reconcile ensures the bootstrap ClusterRole + ClusterRoleBinding exist in
// the consumer workspace where the APIBinding lives. It does not need to read
// the binding's contents — the existence of any binding in the cluster is the
// trigger.
func (r *VWRBACReconciler) Reconcile(ctx context.Context, req mcreconcile.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("cluster", req.ClusterName, "binding", req.Name)

	cl, err := r.manager.GetCluster(ctx, req.ClusterName)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get client for cluster %q: %w", req.ClusterName, err)
	}
	c := cl.GetClient()

	if err := ensureClusterRole(ctx, c); err != nil {
		return ctrl.Result{}, err
	}
	if err := ensureClusterRoleBinding(ctx, c, r.subject); err != nil {
		return ctrl.Result{}, err
	}

	logger.V(2).Info("vwrbac: bootstrap RBAC ensured")
	return ctrl.Result{}, nil
}

func ensureClusterRole(ctx context.Context, c client.Client) error {
	desired := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: roleName},
		Rules: []rbacv1.PolicyRule{
			{
				// Workspace access — required by kcp's workspaceContentAuthorizer
				// for foreign / cross-workspace SAs to even enter the workspace.
				NonResourceURLs: []string{"/"},
				Verbs:           []string{"access"},
			},
			{
				// The "invitation" the apiresourceschema VW authorizer checks.
				APIGroups: []string{apisv1alpha2.SchemeGroupVersion.Group},
				Resources: []string{"apibindings"},
				Verbs:     []string{"get", "list", "watch"},
			},
		},
	}

	existing := &rbacv1.ClusterRole{}
	err := c.Get(ctx, client.ObjectKey{Name: roleName}, existing)
	if errors.IsNotFound(err) {
		if cerr := c.Create(ctx, desired); cerr != nil && !errors.IsAlreadyExists(cerr) {
			return fmt.Errorf("create ClusterRole %q: %w", roleName, cerr)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("get ClusterRole %q: %w", roleName, err)
	}

	// Update only if rules drifted.
	if rulesEqual(existing.Rules, desired.Rules) {
		return nil
	}
	existing.Rules = desired.Rules
	if err := c.Update(ctx, existing); err != nil {
		return fmt.Errorf("update ClusterRole %q: %w", roleName, err)
	}
	return nil
}

func ensureClusterRoleBinding(ctx context.Context, c client.Client, subject rbacv1.Subject) error {
	desired := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: roleName},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     roleName,
		},
		Subjects: []rbacv1.Subject{subject},
	}

	existing := &rbacv1.ClusterRoleBinding{}
	err := c.Get(ctx, client.ObjectKey{Name: roleName}, existing)
	if errors.IsNotFound(err) {
		if cerr := c.Create(ctx, desired); cerr != nil && !errors.IsAlreadyExists(cerr) {
			return fmt.Errorf("create ClusterRoleBinding %q: %w", roleName, cerr)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("get ClusterRoleBinding %q: %w", roleName, err)
	}

	// RoleRef is immutable; only fix subjects if they drifted.
	if subjectsEqual(existing.Subjects, desired.Subjects) {
		return nil
	}
	existing.Subjects = desired.Subjects
	if err := c.Update(ctx, existing); err != nil {
		return fmt.Errorf("update ClusterRoleBinding %q: %w", roleName, err)
	}
	return nil
}

func rulesEqual(a, b []rbacv1.PolicyRule) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !stringSliceEqual(a[i].APIGroups, b[i].APIGroups) ||
			!stringSliceEqual(a[i].Resources, b[i].Resources) ||
			!stringSliceEqual(a[i].Verbs, b[i].Verbs) ||
			!stringSliceEqual(a[i].NonResourceURLs, b[i].NonResourceURLs) ||
			!stringSliceEqual(a[i].ResourceNames, b[i].ResourceNames) {
			return false
		}
	}
	return true
}

func subjectsEqual(a, b []rbacv1.Subject) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func stringSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// SetupWithManager registers the controller with the multicluster-runtime Manager.
func (r *VWRBACReconciler) SetupWithManager(mgr mcmanager.Manager) error {
	return mcbuilder.ControllerManagedBy(mgr).
		For(&apisv1alpha2.APIBinding{}).
		WithOptions(r.opts).
		Named(controllerName).
		Complete(r)
}
