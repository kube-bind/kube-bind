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
	"time"

	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/rest"

	kubebindv1alpha2 "github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2"
	"github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2/helpers"
	conditionsapi "github.com/kube-bind/kube-bind/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
	"github.com/kube-bind/kube-bind/sdk/apis/third_party/conditions/util/conditions"
	bindclient "github.com/kube-bind/kube-bind/sdk/client/clientset/versioned"
)

func (b *BindAPIServiceOptions) createAPIServiceBindings(ctx context.Context, config *rest.Config, request *kubebindv1alpha2.APIServiceExportRequest, secretName string) ([]*kubebindv1alpha2.APIServiceBinding, error) {
	bindClient, err := bindclient.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	apiextensionsClient, err := apiextensionsclientset.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	var bindings []*kubebindv1alpha2.APIServiceBinding
	for _, resource := range request.Spec.Resources {
		name := resource.ResourceGroupName()
		existing, err := bindClient.KubeBindV1alpha2().APIServiceBindings().Get(ctx, name, metav1.GetOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			return nil, err
		} else if err == nil {
			if existing.Spec.KubeconfigSecretRef.Namespace != "kube-bind" || existing.Spec.KubeconfigSecretRef.Name != secretName {
				return nil, fmt.Errorf("found existing APIServiceBinding %s not from this service provider", name)
			}
			fmt.Fprintf(b.Options.IOStreams.ErrOut, "✅ Updating existing APIServiceBinding %s.\n", existing.Name)
			bindings = append(bindings, existing)

			// checking CRD to match the binding
			crd, err := apiextensionsClient.ApiextensionsV1().CustomResourceDefinitions().Get(ctx, resource.ResourceGroupName(), metav1.GetOptions{})
			if err != nil && !apierrors.IsNotFound(err) {
				return nil, err
			} else if err == nil {
				if !helpers.IsOwnedByBinding(existing.Name, existing.UID, crd.OwnerReferences) {
					return nil, fmt.Errorf("CustomResourceDefinition %s exists, but is not owned by kube-bind", crd.Name)
				}
			}
			continue
		}

		// create new APIServiceBinding.
		first := true
		if err := wait.PollUntilContextCancel(ctx, 1*time.Second, false, func(ctx context.Context) (bool, error) {
			if !first {
				first = false
				fmt.Fprint(b.Options.IOStreams.ErrOut, ".")
			}
			created, err := bindClient.KubeBindV1alpha2().APIServiceBindings().Create(ctx, &kubebindv1alpha2.APIServiceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resource.ResourceGroupName(),
					Namespace: "kube-bind",
				},
				Spec: kubebindv1alpha2.APIServiceBindingSpec{
					KubeconfigSecretRef: kubebindv1alpha2.ClusterSecretKeyRef{
						LocalSecretKeyRef: kubebindv1alpha2.LocalSecretKeyRef{
							Name: secretName,
							Key:  "kubeconfig",
						},
						Namespace: "kube-bind",
					},
				},
			}, metav1.CreateOptions{})
			if err != nil {
				return false, err
			}

			// best effort status update to have "Pending" in the Ready condition
			conditions.MarkFalse(created,
				conditionsapi.ReadyCondition,
				"Pending",
				conditionsapi.ConditionSeverityInfo,
				"Pending",
			)
			_, _ = bindClient.KubeBindV1alpha2().APIServiceBindings().UpdateStatus(ctx, created, metav1.UpdateOptions{})

			fmt.Fprintf(b.Options.IOStreams.ErrOut, "✅ Created APIServiceBinding %s.%s\n", resource.Resource, resource.Group)
			bindings = append(bindings, created)
			return true, nil
		}); err != nil {
			fmt.Fprintln(b.Options.IOStreams.ErrOut, "")
			return nil, err
		}
	}

	return bindings, nil
}
