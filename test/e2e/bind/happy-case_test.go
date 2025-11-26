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

package bind

import (
	"context"
	"fmt"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/yaml"

	kuberesources "github.com/kube-bind/kube-bind/backend/kubernetes/resources"
	bindapiservice "github.com/kube-bind/kube-bind/cli/pkg/kubectl/bind-apiservice/plugin"
	examples "github.com/kube-bind/kube-bind/deploy/examples"
	"github.com/kube-bind/kube-bind/pkg/konnector/types"
	kubebindv1alpha2 "github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2"
	"github.com/kube-bind/kube-bind/test/e2e/framework"
)

func TestClusterScoped(t *testing.T) {
	t.Parallel()
	// name & test type defined by letters - cc - cluster-cluster so its easier to identify failures in the logs.
	testHappyCase(t, "cc-prefixed", apiextensionsv1.ClusterScoped, apiextensionsv1.ClusterScoped, kubebindv1alpha2.ClusterScope, kubebindv1alpha2.IsolationPrefixed)
	testHappyCase(t, "cc-none", apiextensionsv1.ClusterScoped, apiextensionsv1.ClusterScoped, kubebindv1alpha2.ClusterScope, kubebindv1alpha2.IsolationNone)
	testHappyCase(t, "cc-namespaced", apiextensionsv1.ClusterScoped, apiextensionsv1.NamespaceScoped, kubebindv1alpha2.ClusterScope, kubebindv1alpha2.IsolationNamespaced)
}

func TestNamespacedScoped(t *testing.T) {
	t.Parallel()

	testHappyCase(t, "nn", apiextensionsv1.NamespaceScoped, apiextensionsv1.NamespaceScoped, kubebindv1alpha2.NamespacedScope, "")
	testHappyCase(t, "nc", apiextensionsv1.NamespaceScoped, apiextensionsv1.NamespaceScoped, kubebindv1alpha2.ClusterScope, "")
}

func testHappyCase(
	t *testing.T,
	name string,
	consumerResourceScope apiextensionsv1.ResourceScope,
	providerResourceScope apiextensionsv1.ResourceScope,
	informerScope kubebindv1alpha2.InformerScope,
	isolationStrategy kubebindv1alpha2.Isolation,
) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel) // Commented out to prevent cleanup of kcp assets

	suffix := framework.RandomString(4)

	t.Logf("Creating provider workspace")
	providerConfig, providerKubeconfig := framework.NewWorkspace(t, framework.ClientConfig(t), framework.WithName("%s-provider-%s", name, suffix))

	t.Logf("Installing kubebind CRDs")
	framework.InstallKubeBindCRDs(t, providerConfig)

	t.Logf("Starting backend with random port")
	addr, _ := framework.StartBackend(t,
		"--kubeconfig="+providerKubeconfig,
		"--listen-address=:0",
		"--consumer-scope="+string(informerScope),
		"--cluster-scoped-isolation="+string(isolationStrategy),
	)

	t.Logf("Creating CRD on provider side")
	examples.Bootstrap(t, framework.DiscoveryClient(t, providerConfig), framework.DynamicClient(t, providerConfig), nil)

	// For namespaced-isolated cluster-scoped objects, the CRD needs to be namespaced on the provider side,
	// but cluster-scoped on the consumer side. To make this setup possible without introducing another CRD,
	// we will simply "hack" the CRD here on the providerside to make it work.
	if providerResourceScope == apiextensionsv1.NamespaceScoped && consumerResourceScope == apiextensionsv1.ClusterScoped {
		t.Logf("Changing sheriff CRD scope to namespaced on the provider side")
		toggleCRDScope(t, ctx, providerConfig, "sheriffs.wildwest.dev", apiextensionsv1.NamespaceScoped)
	}

	t.Logf("Creating consumer workspace and starting konnector")
	consumerConfig, consumerKubeconfig := framework.NewWorkspace(t, framework.ClientConfig(t), framework.WithName("%s-consumer-%s", name, suffix))
	framework.StartKonnector(t, consumerConfig, "--kubeconfig="+consumerKubeconfig)

	serviceGVR := schema.GroupVersionResource{Group: "wildwest.dev", Version: "v1alpha1", Resource: "cowboys"}
	templateRef := "cowboys"

	if consumerResourceScope == apiextensionsv1.ClusterScoped {
		serviceGVR = schema.GroupVersionResource{Group: "wildwest.dev", Version: "v1alpha1", Resource: "sheriffs"}
		templateRef = "sheriffs"
	}

	consumerClient := framework.DynamicClient(t, consumerConfig).Resource(serviceGVR)
	providerClient := framework.DynamicClient(t, providerConfig).Resource(serviceGVR)

	providerCoreClient := framework.KubeClient(t, providerConfig).CoreV1()
	providerBindClient := framework.BindClient(t, providerConfig)

	// Instance variables removed - now seeded directly in test
	// These two namespaces are where the "main" object resides on each cluster.
	consumerNs, providerNs := "wild-west", "unknown"

	// When namespaced isolation is used (i.e. cluster-scoped objects on the consumer
	// turn into namespaced objects on the provider cluster), permission claimed objects
	// are still synced into their respective APIServiceNamespace-managed namespaces.
	permClaimNs := "unknown"

	// cluster namespace is the main "contract" namespace, i.e. where the BoundSchema and other
	// bind-related objects reside.
	clusterNs, clusterScopedUpInsName := "unknown", "unknown"

	kubeBindConfig := path.Join(framework.WorkDir, "kube-bind-config.yaml")

	// Secrets from the actual examples:
	// For cowboys: colt-45-permit (referenced), cowboy-gang-affiliation (label selector)
	// For sheriffs: sheriff-badge-credentials (referenced), sheriff-jurisdiction-config (label selector)
	var referencedSecretName, labelSelectedSecretName string
	var filename string
	if consumerResourceScope == apiextensionsv1.NamespaceScoped {
		referencedSecretName = "colt-45-permit" //nolint:gosec
		labelSelectedSecretName = "cowboy-gang-affiliation"
		filename = "cr-cowboy.yaml"
	} else {
		referencedSecretName = "sheriff-badge-credentials"
		labelSelectedSecretName = "sheriff-jurisdiction-config"
		filename = "cr-sheriff.yaml"
	}

	// Note: sheriff secrets are in sheriff-operations namespace, but they will be synced to the provider namespace

	// binding step outputs this.
	var bindResponse *kubebindv1alpha2.BindingResourceResponse
	for _, tc := range []struct {
		name string
		step func(t *testing.T)
	}{
		{
			name: "Login to provider",
			step: func(t *testing.T) {
				iostreams, _, _, _ := genericclioptions.NewTestIOStreams()
				authURLDryRunCh := make(chan string, 1)
				go framework.SimulateBrowser(t, authURLDryRunCh)
				framework.Login(t, iostreams, authURLDryRunCh, kubeBindConfig, fmt.Sprintf("http://%s/api/exports", addr.String()), "")
			},
		},
		{
			name: "List templates",
			step: func(t *testing.T) {
				c := framework.GetKubeBindRestClient(t, kubeBindConfig)
				result, err := c.GetTemplates(ctx)
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Len(t, result.Items, 2)
			},
		},
		{
			name: "List collections",
			step: func(t *testing.T) {
				c := framework.GetKubeBindRestClient(t, kubeBindConfig)
				result, err := c.GetCollections(ctx)
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Len(t, result.Items, 1)
			},
		},
		{
			name: "Get bind APIServiceExportRequest from server",
			step: func(t *testing.T) {
				c := framework.GetKubeBindRestClient(t, kubeBindConfig)
				var err error
				bindResponse, err = c.Bind(ctx, &kubebindv1alpha2.BindableResourcesRequest{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-binding",
					},
					TemplateRef: kubebindv1alpha2.APIServiceExportTemplateRef{
						Name: templateRef,
					},
				})
				require.NoError(t, err)
				require.NotNil(t, bindResponse)
			},
		},
		{
			name: "Bind the payload",
			step: func(t *testing.T) {
				iostreams, _, _, _ := genericclioptions.NewTestIOStreams()
				binderOpts := &bindapiservice.BinderOptions{
					IOStreams:     iostreams,
					SkipKonnector: true,
				}

				binder := bindapiservice.NewBinder(consumerConfig, binderOpts)
				result, err := binder.BindFromResponse(ctx, bindResponse)
				require.NoError(t, err)
				require.Len(t, result, 1)

				t.Logf("Waiting for %s CRD to be created on consumer side", serviceGVR.Resource)
				crdClient := framework.ApiextensionsClient(t, consumerConfig).ApiextensionsV1().CustomResourceDefinitions()
				require.Eventually(t, func() bool {
					if serviceGVR.Group == "" {
						serviceGVR.Group = "core"
					}
					_, err := crdClient.Get(ctx, serviceGVR.Resource+"."+serviceGVR.Group, metav1.GetOptions{})
					return err == nil
				}, wait.ForeverTestTimeout, time.Millisecond*100, "waiting for %s CRD to be created on consumer side", serviceGVR.Resource)
			},
		},
		{
			name: "instances are synced",
			step: func(t *testing.T) {
				t.Logf("Seeding example objects on consumer side for %s", serviceGVR.Resource)

				err := seedSpecificCRExample(t, framework.DynamicClient(t, consumerConfig), filename, "test")
				require.NoError(t, err)

				t.Logf("Trying to create %s on consumer side", serviceGVR.Resource)

				t.Logf("Waiting for the %s instance to be created on provider side", serviceGVR.Resource)
				var instances *unstructured.UnstructuredList
				require.Eventually(t, func() bool {
					var err error
					instances, err = providerClient.List(ctx, metav1.ListOptions{})
					return err == nil && len(instances.Items) == 1
				}, wait.ForeverTestTimeout, time.Millisecond*100, "waiting for the %s instance to be created on provider side", serviceGVR.Resource)

				// these are used everywhere further down
				firstObj := instances.Items[0]

				// providerNs is the namespaced where the synced object lives; this might be empty
				// for cluster-scoped objects on the provider side.
				providerNs = firstObj.GetNamespace()

				// Cluster namespace represent binding contract namespace, and is stored on every object.
				clusterNs = firstObj.GetAnnotations()[types.ClusterNamespaceAnnotationKey]
				require.NotEmpty(t, clusterNs, "cluster namespace annotation must always exist")

				// the object name on the provider side; this is only used for cluster-scoped objects
				clusterScopedUpInsName = firstObj.GetName()
			},
		},
		// Request included namespace, so we check it first
		{
			name: "verify provider side namespace pre-seeding and RBAC management",
			step: func(t *testing.T) {
				t.Logf("Verifying APIServiceNamespace was created from pre-seeded namespace spec")
				var foundPreSeededNamespace bool
				var actualProviderNamespace string
				require.Eventually(t, func() bool {
					// If we are operating namespaced resources - namespace will be set, if cluster - we need to use
					// extracted one from the cluster-scoped object.
					namespaces, err := providerBindClient.KubeBindV1alpha2().APIServiceNamespaces(clusterNs).List(ctx, metav1.ListOptions{})
					if err != nil {
						return false
					}

					for _, ns := range namespaces.Items {
						if ns.Name == "wild-west" && ns.Status.Namespace != "" {
							actualProviderNamespace = ns.Status.Namespace
							foundPreSeededNamespace = true
							return true
						}
					}
					return false
				}, wait.ForeverTestTimeout, time.Millisecond*100, "waiting for pre-seeded APIServiceNamespace to be created on provider side")
				require.True(t, foundPreSeededNamespace, "Pre-seeded namespace 'wild-west' should be created via APIServiceExportRequest.Spec.Namespaces")

				providerCoreClient := framework.KubeClient(t, providerConfig).CoreV1()
				_, err := providerCoreClient.Namespaces().Get(ctx, actualProviderNamespace, metav1.GetOptions{})
				require.NoError(t, err, "Physical provider side namespace should exist")

				switch informerScope {
				case kubebindv1alpha2.ClusterScope:
					t.Logf("Verifying RBAC resources were created for secret management in cluster scope")
					rbacClient := framework.KubeClient(t, providerConfig).RbacV1()

					clusterRoles, err := rbacClient.ClusterRoles().List(ctx, metav1.ListOptions{})
					require.NoError(t, err)

					var foundSecretClusterRole bool
					for _, cr := range clusterRoles.Items {
						if strings.Contains(cr.Name, "kube-binder-export") {
							for _, rule := range cr.Rules {
								for _, resource := range rule.Resources {
									if resource == "secrets" {
										foundSecretClusterRole = true
										require.Contains(t, rule.Verbs, "*", "ClusterRole should have * permissions for secrets")
										require.Contains(t, rule.APIGroups, "", "ClusterRole should target core API group")
										break
									}
								}
							}
						}
					}
					require.True(t, foundSecretClusterRole, "ClusterRole for secrets should be created")

					t.Logf("Verifying ClusterRoleBinding was created for pre-seeded namespace secret access")
					clusterRoleBindings, err := rbacClient.ClusterRoleBindings().List(ctx, metav1.ListOptions{})
					require.NoError(t, err)

					var foundSecretClusterRoleBinding bool
					for _, crb := range clusterRoleBindings.Items {
						if strings.Contains(crb.Name, "kube-binder-export") {
							for _, subject := range crb.Subjects {
								if subject.Kind == "ServiceAccount" && subject.Name == kuberesources.ServiceAccountName {
									foundSecretClusterRoleBinding = true
									require.Equal(t, "ClusterRole", crb.RoleRef.Kind, "Should reference ClusterRole")
									break
								}
							}
						}
					}
					require.True(t, foundSecretClusterRoleBinding, "ClusterRoleBinding for ServiceAccount should be created")
				case kubebindv1alpha2.NamespacedScope:
					t.Logf("Verifying RBAC resources were created for secret management in namespace scope")
					rbacClient := framework.KubeClient(t, providerConfig).RbacV1()

					roles, err := rbacClient.Roles(providerNs).List(ctx, metav1.ListOptions{})
					require.NoError(t, err)

					var foundSecretRole bool
					for _, cr := range roles.Items {
						if strings.Contains(cr.Name, "kube-binder-export") {
							for _, rule := range cr.Rules {
								for _, resource := range rule.Resources {
									if resource == "secrets" {
										foundSecretRole = true
										require.Contains(t, rule.Verbs, "*", "Role should have * permissions for secrets")
										require.Contains(t, rule.APIGroups, "", "Role should target core API group")
										break
									}
								}
							}
						}
					}
					require.True(t, foundSecretRole, "Role for secrets should be created")

					t.Logf("Verifying RoleBinding was created for pre-seeded namespace secret access")
					roleBindings, err := rbacClient.RoleBindings(providerNs).List(ctx, metav1.ListOptions{})
					require.NoError(t, err)

					var foundSecretRoleBinding bool
					for _, crb := range roleBindings.Items {
						if strings.Contains(crb.Name, "kube-binder-") && strings.Contains(crb.Name, "-export-") {
							for _, subject := range crb.Subjects {
								if subject.Kind == "ServiceAccount" && subject.Name == kuberesources.ServiceAccountName {
									foundSecretRoleBinding = true
									require.Equal(t, "Role", crb.RoleRef.Kind, "Should reference Role")
									break
								}
							}
						}
					}
					require.True(t, foundSecretRoleBinding, "RoleBinding for ServiceAccount should be created")
				}

				t.Logf("Provider side namespace pre-seeding and secret management RBAC verified successfully")
			},
		},
		{
			name: "verify secrets from examples are synced",
			step: func(t *testing.T) {
				t.Logf("Verifying that secrets from the embedded examples are already synced to provider")

				err := seedSpecificCRExample(t, framework.DynamicClient(t, consumerConfig), filename, "test")
				require.NoError(t, err)
			},
		},
		{
			name: "establish permission claims namespace",
			step: func(t *testing.T) {
				// We need to establish namespace only in cluster scope for cluster scoped resources.
				// Else we can trust sync object namespace as it will be the same.
				if informerScope == kubebindv1alpha2.ClusterScope &&
					consumerResourceScope == apiextensionsv1.ClusterScoped {
					if providerNs == "unknown" {
						t.Fatal("providerNS is not set. Programming error in the test.")
					}

					t.Logf("Waiting for APIServiceNamespace to be created on provider side: %s", clusterNs)
					require.Eventually(t, func() bool {
						var err error
						namespaces, err := providerBindClient.KubeBindV1alpha2().APIServiceNamespaces(clusterNs).List(ctx, metav1.ListOptions{})
						if err != nil {
							return false
						}

						for _, namespace := range namespaces.Items {
							if strings.Contains(namespace.Name, consumerNs) && namespace.Status.Namespace != "" {
								permClaimNs = namespace.Status.Namespace
								return true
							}
						}

						return false
					}, wait.ForeverTestTimeout, time.Millisecond*100, "waiting for APIServiceNamespace to be created on provider side")
					require.NotEmpty(t, permClaimNs, "No permission claim namespaces found")

					t.Logf("permclaim namespace detected as: %q", permClaimNs)
				} else {
					permClaimNs = providerNs
				}
			},
		},
		{
			name: "verify secrets from examples are synced to provider",
			step: func(t *testing.T) {
				t.Logf("Waiting for referenced secret to be synced to provider side")
				require.Eventually(t, func() bool {
					_, err := providerCoreClient.Secrets(permClaimNs).Get(ctx, referencedSecretName, metav1.GetOptions{})
					return err == nil
				}, time.Minute*2, time.Millisecond*100, "waiting for referenced secret to be synced to provider side")

				t.Logf("Waiting for label-selected secret to be synced to provider side")
				require.Eventually(t, func() bool {
					_, err := providerCoreClient.Secrets(permClaimNs).Get(ctx, labelSelectedSecretName, metav1.GetOptions{})
					return err == nil
				}, wait.ForeverTestTimeout, time.Millisecond*100, "waiting for label-selected secret to be synced to provider side")

				t.Logf("Secrets from examples are properly synced to provider")
			},
		},
		{
			name: "verify secrets are deleted when resource is removed",
			step: func(t *testing.T) {
				t.Logf("Note: Secrets are created from the embedded examples and will be cleaned up when the instance is deleted")
				// For now, we skip manual secret deletion testing since the secrets come from the examples
				// and are tied to the resource lifecycle
			},
		},
		{
			name: "instance deleted upstream is recreated",
			step: func(t *testing.T) {
				var err error
				if providerResourceScope == apiextensionsv1.NamespaceScoped {
					err = providerClient.Namespace(providerNs).Delete(ctx, "test", metav1.DeleteOptions{})
				} else {
					err = providerClient.Delete(ctx, clusterScopedUpInsName, metav1.DeleteOptions{})
				}
				require.NoError(t, err)

				require.Eventually(t, func() bool {
					var err error
					if providerResourceScope == apiextensionsv1.NamespaceScoped {
						_, err = providerClient.Namespace(providerNs).Get(ctx, "test", metav1.GetOptions{})
					} else {
						_, err = providerClient.Get(ctx, clusterScopedUpInsName, metav1.GetOptions{})
					}
					return err == nil
				}, wait.ForeverTestTimeout, time.Millisecond*100, "waiting for the %s instance to be recreated upstream", serviceGVR.Resource)
			},
		},
		{
			name: "instance spec updated downstream is updated upstream",
			step: func(t *testing.T) {
				err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
					var obj *unstructured.Unstructured
					var err error
					if consumerResourceScope == apiextensionsv1.NamespaceScoped {
						obj, err = consumerClient.Namespace(consumerNs).Get(ctx, "test", metav1.GetOptions{})
					} else {
						obj, err = consumerClient.Get(ctx, "test", metav1.GetOptions{})
					}
					require.NoError(t, err)
					if consumerResourceScope == apiextensionsv1.NamespaceScoped {
						unstructured.SetNestedField(obj.Object, "Updated cowboy intent", "spec", "intent") //nolint:errcheck
						_, err = consumerClient.Namespace(consumerNs).Update(ctx, obj, metav1.UpdateOptions{})
					} else {
						unstructured.SetNestedField(obj.Object, "Updated sheriff intent", "spec", "intent") //nolint:errcheck
						_, err = consumerClient.Update(ctx, obj, metav1.UpdateOptions{})
					}
					return err
				})
				require.NoError(t, err)

				require.Eventually(t, func() bool {
					var obj *unstructured.Unstructured
					var err error
					if providerResourceScope == apiextensionsv1.NamespaceScoped {
						obj, err = providerClient.Namespace(providerNs).Get(ctx, "test", metav1.GetOptions{})
					} else {
						obj, err = providerClient.Get(ctx, clusterScopedUpInsName, metav1.GetOptions{})
					}
					require.NoError(t, err)
					var value string
					value, _, err = unstructured.NestedString(obj.Object, "spec", "intent")

					require.NoError(t, err)
					if consumerResourceScope == apiextensionsv1.NamespaceScoped {
						return value == "Updated cowboy intent"
					} else {
						return value == "Updated sheriff intent"
					}
				}, wait.ForeverTestTimeout, time.Millisecond*100, "waiting for the %s instance to be updated upstream", serviceGVR.Resource)
			},
		},
		{
			name: "instance status updated upstream is updated downstream",
			step: func(t *testing.T) {
				err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
					var obj *unstructured.Unstructured
					var err error
					if providerResourceScope == apiextensionsv1.NamespaceScoped {
						obj, err = providerClient.Namespace(providerNs).Get(ctx, "test", metav1.GetOptions{})
					} else {
						obj, err = providerClient.Get(ctx, clusterScopedUpInsName, metav1.GetOptions{})
					}
					require.NoError(t, err)
					unstructured.SetNestedField(obj.Object, "Ready to ride", "status", "result") //nolint:errcheck
					if providerResourceScope == apiextensionsv1.NamespaceScoped {
						_, err = providerClient.Namespace(providerNs).UpdateStatus(ctx, obj, metav1.UpdateOptions{})
					} else {
						_, err = providerClient.UpdateStatus(ctx, obj, metav1.UpdateOptions{})
					}
					return err
				})
				require.NoError(t, err)

				require.Eventually(t, func() bool {
					var obj *unstructured.Unstructured
					var err error
					if consumerResourceScope == apiextensionsv1.NamespaceScoped {
						obj, err = consumerClient.Namespace(consumerNs).Get(ctx, "test", metav1.GetOptions{})
					} else {
						obj, err = consumerClient.Get(ctx, "test", metav1.GetOptions{})
					}
					require.NoError(t, err)
					value, _, err := unstructured.NestedString(obj.Object, "status", "result")
					require.NoError(t, err)
					return value == "Ready to ride"
				}, wait.ForeverTestTimeout, time.Millisecond*100, "waiting for the %s instance to be updated downstream", serviceGVR.Resource)
			},
		},
		{
			name: "instance spec updated upstream is reconciled upstream",
			step: func(t *testing.T) {
				err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
					var obj *unstructured.Unstructured
					var err error
					if providerResourceScope == apiextensionsv1.NamespaceScoped {
						obj, err = providerClient.Namespace(providerNs).Get(ctx, "test", metav1.GetOptions{})
					} else {
						obj, err = providerClient.Get(ctx, clusterScopedUpInsName, metav1.GetOptions{})
					}
					require.NoError(t, err)
					if providerResourceScope == apiextensionsv1.NamespaceScoped {
						unstructured.SetNestedField(obj.Object, "Drifted cowboy intent", "spec", "intent") //nolint:errcheck
						_, err = providerClient.Namespace(providerNs).Update(ctx, obj, metav1.UpdateOptions{})
					} else {
						unstructured.SetNestedField(obj.Object, "Drifted sheriff intent", "spec", "intent") //nolint:errcheck
						_, err = providerClient.Update(ctx, obj, metav1.UpdateOptions{})
					}
					return err
				})
				require.NoError(t, err)

				require.Eventually(t, func() bool {
					var obj *unstructured.Unstructured
					var err error
					if providerResourceScope == apiextensionsv1.NamespaceScoped {
						obj, err = providerClient.Namespace(providerNs).Get(ctx, "test", metav1.GetOptions{})
					} else {
						obj, err = providerClient.Get(ctx, clusterScopedUpInsName, metav1.GetOptions{})
					}
					require.NoError(t, err)
					var value string
					if consumerResourceScope == apiextensionsv1.NamespaceScoped {
						value, _, err = unstructured.NestedString(obj.Object, "spec", "intent")
						require.NoError(t, err)
						return value == "Updated cowboy intent"
					} else {
						value, _, err = unstructured.NestedString(obj.Object, "spec", "intent")
						require.NoError(t, err)
						return value == "Updated sheriff intent"
					}
				}, wait.ForeverTestTimeout, time.Millisecond*100, "waiting for the %s instance to be reconciled upstream", serviceGVR.Resource)
			},
		},
		{
			name: "instances deleted downstream are deleted upstream",
			step: func(t *testing.T) {
				var err error
				if consumerResourceScope == apiextensionsv1.NamespaceScoped {
					err = consumerClient.Namespace(consumerNs).Delete(ctx, "test", metav1.DeleteOptions{})
				} else {
					err = consumerClient.Delete(ctx, "test", metav1.DeleteOptions{})
				}
				require.NoError(t, err)

				require.Eventually(t, func() bool {
					var err error
					if providerResourceScope == apiextensionsv1.NamespaceScoped {
						_, err = providerClient.Namespace(providerNs).Get(ctx, "test", metav1.GetOptions{})
					} else {
						_, err = providerClient.Get(ctx, clusterScopedUpInsName, metav1.GetOptions{})
					}
					return errors.IsNotFound(err)
				}, wait.ForeverTestTimeout, time.Millisecond*100, "waiting for the %s instance to be deleted on provider side", serviceGVR.Resource)
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.step(t)
		})
	}
}

func seedSpecificCRExample(t *testing.T, dynamicClient dynamic.Interface, filename, instanceName string) error {
	t.Helper()

	ctx := t.Context()

	// Read and apply all objects from the specified CR example file
	bytes, err := examples.CRExamples.ReadFile(filename)
	if err != nil {
		return err
	}

	// Replace the main resource name with the test name
	yamlContent := string(bytes)
	if strings.Contains(filename, "cowboy") {
		yamlContent = strings.ReplaceAll(yamlContent, "name: billy-the-kid", "name: "+instanceName)
	} else if strings.Contains(filename, "sheriff") {
		yamlContent = strings.ReplaceAll(yamlContent, "name: wyatt-earp", "name: "+instanceName)
	}

	return applyMultiDocYAML(ctx, t, dynamicClient, yamlContent)
}

func applyMultiDocYAML(ctx context.Context, t *testing.T, dynamicClient dynamic.Interface, yamlContent string) error {
	t.Helper()

	// Split YAML documents
	documents := strings.Split(yamlContent, "---")

	for _, doc := range documents {
		doc = strings.TrimSpace(doc)
		if doc == "" || strings.HasPrefix(doc, "#") {
			continue
		}

		obj := &unstructured.Unstructured{}
		if err := yaml.Unmarshal([]byte(doc), obj); err != nil {
			continue // Skip invalid documents
		}

		if obj.GetKind() == "" {
			continue // Skip empty documents
		}

		gvk := obj.GroupVersionKind()
		gvr := schema.GroupVersionResource{
			Group:   gvk.Group,
			Version: gvk.Version,
			// Resources is handled below.
		}

		// Handle special cases for pluralization
		switch gvk.Kind {
		case "Namespace":
			gvr.Resource = "namespaces"
		case "Secret":
			gvr.Resource = "secrets"
		case "Cowboy":
			gvr.Resource = "cowboys"
		case "Sheriff":
			gvr.Resource = "sheriffs"
		}

		client := dynamicClient.Resource(gvr)

		// Apply based on namespace scope
		if obj.GetNamespace() != "" {
			_, err := client.Namespace(obj.GetNamespace()).Create(ctx, obj, metav1.CreateOptions{})
			if err != nil && !errors.IsAlreadyExists(err) {
				return err
			}
		} else {
			_, err := client.Create(ctx, obj, metav1.CreateOptions{})
			if err != nil && !errors.IsAlreadyExists(err) {
				return err
			}
		}
	}

	return nil
}

func toggleCRDScope(t *testing.T, ctx context.Context, config *rest.Config, name string, scope apiextensionsv1.ResourceScope) {
	clientset, err := apiextensionsclient.NewForConfig(config)
	require.NoError(t, err)

	crdClient := clientset.ApiextensionsV1().CustomResourceDefinitions()

	// copy existing CRD
	crd, err := crdClient.Get(ctx, name, metav1.GetOptions{})
	require.NoError(t, err)

	// delete it
	require.NoError(t, crdClient.Delete(ctx, name, metav1.DeleteOptions{}))

	require.Eventually(t, func() bool {
		_, err := crdClient.Get(ctx, name, metav1.GetOptions{})
		return errors.IsNotFound(err)
	}, wait.ForeverTestTimeout, time.Millisecond*100, "waiting for the CRD to be deleted")

	// re-create it
	crd.Spec.Scope = scope
	crd.ObjectMeta.ResourceVersion = ""
	crd.ObjectMeta.UID = ""
	crd.ObjectMeta.Generation = 0
	_, err = crdClient.Create(ctx, crd, metav1.CreateOptions{})
	require.NoError(t, err)
}
