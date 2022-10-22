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

package plugin

import (
	"context"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	kubeclient "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	kubebindv1alpha1 "github.com/kube-bind/kube-bind/pkg/apis/kubebind/v1alpha1"
	conditionsapi "github.com/kube-bind/kube-bind/pkg/apis/third_party/conditions/apis/conditions/v1alpha1"
	"github.com/kube-bind/kube-bind/pkg/apis/third_party/conditions/util/conditions"
	bindclient "github.com/kube-bind/kube-bind/pkg/client/clientset/versioned"
	"github.com/kube-bind/kube-bind/pkg/kubectl/base"
)

// nolint: unused
func (b *BindAPIServiceOptions) createAPIServiceBindings(ctx context.Context, config *rest.Config, request *kubebindv1alpha1.APIServiceBindingRequest, remoteHost, remoteNamespace, kubeconfig string) ([]*kubebindv1alpha1.APIServiceBinding, error) {
	logger := klog.FromContext(ctx).WithValues("request", request.Name, "remoteHost", remoteHost, "remoteNamespace", remoteNamespace)

	bindClient, err := bindclient.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	kubeClient, err := kubeclient.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	apiextensionsClient, err := apiextensionsclientset.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	// create kube-bind namespace
	if _, err := kubeClient.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "kube-bind",
		},
	}, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
		return nil, err
	} else if err == nil {
		fmt.Fprintf(b.Options.IOStreams.ErrOut, "Created kube-binding namespace.\n") // nolint: errcheck
	}

	// look for secret of the given identity
	secretName, err := base.FindRemoteKubeconfig(ctx, kubeClient, remoteNamespace, remoteHost)
	if err != nil {
		return nil, err
	}

	var bindings []*kubebindv1alpha1.APIServiceBinding
nextRequest:
	for _, resource := range request.Spec.Resources {
		crd, err := apiextensionsClient.ApiextensionsV1().CustomResourceDefinitions().Get(ctx, resource.Resource+"."+resource.Group, metav1.GetOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			return nil, err
		}
		if secretName == "" {
			if err == nil {
				return nil, fmt.Errorf("CRD %s.%s already exists and is not from this service provider", resource.Resource, resource.Group)
			}

			fmt.Fprintf(b.Options.IOStreams.ErrOut, "Creating secret for host %s, namespace %s\n", remoteHost, remoteNamespace) // nolint: errcheck
			secretName, err = b.ensureKubeconfigSecretWithLogging(ctx, kubeconfig, "", kubeClient)
			if err != nil {
				return nil, err
			}
		} else if err == nil {
			logger.V(1).Info("Found existing CRD, checking owner", "crd", crd.Name) // noline: errcheck

			// check if the CRD is owner-refed by the APIServiceBinding
			for _, ref := range crd.OwnerReferences {
				parts := strings.SplitN(ref.APIVersion, "/", 2)
				if parts[0] != kubebindv1alpha1.SchemeGroupVersion.Group || ref.Kind != "APIServiceBinding" {
					continue
				}

				existing, err := bindClient.KubeBindV1alpha1().APIServiceBindings().Get(ctx, ref.Name, metav1.GetOptions{})
				if err != nil && !apierrors.IsNotFound(err) {
					return nil, err
				} else if apierrors.IsNotFound(err) {
					continue
				}

				if existing.Spec.KubeconfigSecretRef.Namespace == "kube-bind" && existing.Spec.KubeconfigSecretRef.Name == secretName {
					fmt.Fprintf(b.Options.IOStreams.ErrOut, "Updating kubeconfig in secret kube-bind/%s for existing APIServiceBinding %s.\n", secretName, existing.Name) // nolint: errcheck
					if _, err = b.ensureKubeconfigSecretWithLogging(ctx, kubeconfig, secretName, kubeClient); err != nil {
						return nil, err
					}
					bindings = append(bindings, existing)
					continue nextRequest
				}
			}
			return nil, fmt.Errorf("found existing CustomResourceDefinition %s not from this service provider host %s, namespace %s", crd.Name, remoteHost, remoteNamespace)
		} else {
			secretName, err = b.ensureKubeconfigSecretWithLogging(ctx, kubeconfig, secretName, kubeClient)
			if err != nil {
				return nil, err
			}
		}

		// create new APIServiceBinding.
		first := true
		if err := wait.PollInfinite(1*time.Second, func() (bool, error) {
			if !first {
				first = false
				fmt.Fprint(b.Options.IOStreams.ErrOut, ".") // nolint: errcheck
			}
			created, err := bindClient.KubeBindV1alpha1().APIServiceBindings().Create(ctx, &kubebindv1alpha1.APIServiceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resource.Resource + "." + resource.Group,
					Namespace: "kube-bind",
				},
				Spec: kubebindv1alpha1.APIServiceBindingSpec{
					KubeconfigSecretRef: kubebindv1alpha1.ClusterSecretKeyRef{
						LocalSecretKeyRef: kubebindv1alpha1.LocalSecretKeyRef{
							Name: secretName,
							Key:  "kubeconfig",
						},
						Namespace: "kube-bind",
					},
				},
			}, metav1.CreateOptions{})
			if err != nil && !apierrors.IsAlreadyExists(err) {
				return false, err
			} else if apierrors.IsAlreadyExists(err) {
				existing, err := bindClient.KubeBindV1alpha1().APIServiceBindings().Get(ctx, resource.Resource+"."+resource.Group, metav1.GetOptions{})
				if err != nil {
					return false, nil
				}
				if existing.Spec.KubeconfigSecretRef.Namespace == "kube-bind" && existing.Spec.KubeconfigSecretRef.Name == secretName {
					fmt.Fprintf(b.Options.IOStreams.ErrOut, "Adopting APIServiceBinding %s.%s\n", resource.Resource, resource.Group) // nolint: errcheck
					bindings = append(bindings, existing)
					return true, nil
				}

				return false, fmt.Errorf("APIServiceBinding %s.%s already exists, but from different provider", resource.Resource, resource.Group)
			}

			// best effort status update to have "Pending" in the Ready condition
			conditions.MarkFalse(created,
				conditionsapi.ReadyCondition,
				"Pending",
				conditionsapi.ConditionSeverityInfo,
				"Pending",
			)
			_, _ = bindClient.KubeBindV1alpha1().APIServiceBindings().UpdateStatus(ctx, created, metav1.UpdateOptions{}) // nolint:errcheck

			fmt.Fprintf(b.Options.IOStreams.ErrOut, "Created APIServiceBinding %s.%s\n", resource.Resource, resource.Group) // nolint: errcheck
			bindings = append(bindings, created)
			return true, nil
		}); err != nil {
			fmt.Fprintln(b.Options.IOStreams.ErrOut, "") // nolint: errcheck
			return nil, err
		}
	}

	return bindings, nil
}

func (b *BindAPIServiceOptions) ensureKubeconfigSecretWithLogging(ctx context.Context, kubeconfig, name string, client kubeclient.Interface) (string, error) {
	secret, created, err := base.EnsureKubeconfigSecret(ctx, kubeconfig, name, client)
	if err != nil {
		return "", err
	}

	remoteHost, remoteNamespace, err := base.ParseRemoteKubeconfig([]byte(kubeconfig))
	if err != nil {
		return "", err
	}

	if b.remoteKubeconfigFile != "" {
		if created {
			fmt.Fprintf(b.Options.ErrOut, "Created secret %s/%s for host %s, namespace %s\n", "kube-bind", secret.Name, remoteHost, remoteNamespace)
		} else {
			fmt.Fprintf(b.Options.ErrOut, "Updated secret %s/%s for host %s, namespace %s\n", "kube-bind", secret.Name, remoteHost, remoteNamespace)
		}
	}

	return secret.Name, nil
}
