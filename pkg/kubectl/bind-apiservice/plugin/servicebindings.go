/*
Copyright 2023 The Kube Bind Authors.

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
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/rest"

	kubebindv1alpha1 "github.com/kube-bind/kube-bind/pkg/apis/kubebind/v1alpha1"
	"github.com/kube-bind/kube-bind/pkg/apis/kubebind/v1alpha1/helpers"
	conditionsapi "github.com/kube-bind/kube-bind/pkg/apis/third_party/conditions/apis/conditions/v1alpha1"
	"github.com/kube-bind/kube-bind/pkg/apis/third_party/conditions/util/conditions"
	bindclient "github.com/kube-bind/kube-bind/pkg/client/clientset/versioned"
)

func (b *BindAPIServiceOptions) createAPIServiceBindings(ctx context.Context, config *rest.Config, request *kubebindv1alpha1.APIServiceExportRequest, secretName string) ([]*kubebindv1alpha1.APIServiceBinding, error) {
	bindClient, err := bindclient.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	apiextensionsClient, err := apiextensionsclientset.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	var bindings []*kubebindv1alpha1.APIServiceBinding
	for _, resource := range request.Spec.Resources {
		name := resource.Resource + "." + resource.Group
		existing, err := bindClient.KubeBindV1alpha1().APIServiceBindings().Get(ctx, name, metav1.GetOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			return nil, err
		} else if err == nil {
			if existing.Spec.KubeconfigSecretRef.Namespace != "kube-bind" || existing.Spec.KubeconfigSecretRef.Name != secretName {
				return nil, fmt.Errorf("found existing APIServiceBinding %s not from this service provider", name)
			}
			fmt.Fprintf(b.Options.IOStreams.ErrOut, "✅ Updating existing APIServiceBinding %s.\n", existing.Name) // nolint: errcheck
			bindings = append(bindings, existing)

			// checking CRD to match the binding
			crd, err := apiextensionsClient.ApiextensionsV1().CustomResourceDefinitions().Get(ctx, resource.Resource+"."+resource.Group, metav1.GetOptions{})
			if err != nil && !apierrors.IsNotFound(err) {
				return nil, err
			} else if err == nil {
				if !helpers.IsOwnedByBinding(existing.Name, existing.UID, crd.OwnerReferences) {
					return nil, fmt.Errorf("CustomResourceDefinition %s exists, but is not owned by kube-bind", crd.Name)
				}
			}
			continue
		}

		var permissionClaims []kubebindv1alpha1.AcceptablePermissionClaim
		for _, c := range resource.PermissionClaims {
			accepted, err := b.promptYesNo(c)
			if err != nil {
				return nil, err
			}

			var state kubebindv1alpha1.AcceptablePermissionClaimState
			if accepted {
				state = kubebindv1alpha1.ClaimAccepted
			} else {
				state = kubebindv1alpha1.ClaimRejected
			}

			permissionClaims = append(permissionClaims, kubebindv1alpha1.AcceptablePermissionClaim{
				PermissionClaim: c,
				State:           state,
			})
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
					PermissionClaims: permissionClaims,
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
			_, _ = bindClient.KubeBindV1alpha1().APIServiceBindings().UpdateStatus(ctx, created, metav1.UpdateOptions{}) // nolint:errcheck

			fmt.Fprintf(b.Options.IOStreams.ErrOut, "✅ Created APIServiceBinding %s.%s\n", resource.Resource, resource.Group) // nolint: errcheck
			bindings = append(bindings, created)
			return true, nil
		}); err != nil {
			fmt.Fprintln(b.Options.IOStreams.ErrOut, "") // nolint: errcheck
			return nil, err
		}
	}

	return bindings, nil
}

func printPermissionClaim(w io.Writer, p kubebindv1alpha1.PermissionClaim) error {
	var b bytes.Buffer

	var groupResource string
	if p.GroupResource.Group != "" {
		groupResource = fmt.Sprintf("%s objects (apiVersion: \"%s/%s\")", p.GroupResource.Resource, p.GroupResource.Group, p.Version)
	} else {
		groupResource = fmt.Sprintf("%s objects (apiVersion: \"%s\")", p.GroupResource.Resource, p.Version)
	}

	if err := writeFirstLines(&b, groupResource, p); err != nil {
		return err
	}

	if err := writeOnConflict(&b, p); err != nil {
		return err
	}

	if err := writeUpdateClause(&b, p); err != nil {
		return err
	}

	if err := writeRequiredAndAcceptance(&b, p.Required); err != nil {
		return err
	}

	_, err := fmt.Fprint(w, b.String())
	return err
}

func writeFirstLines(b *bytes.Buffer, groupResource string, claim kubebindv1alpha1.PermissionClaim) error {
	var err error

	donate := false
	if claim.Create != nil {
		donate = claim.Create.Donate
	}
	adopt := claim.Adopt

	name := ""
	var owner kubebindv1alpha1.Owner
	if (claim.Selector != kubebindv1alpha1.ResourceSelector{}) {
		name = claim.Selector.Name
		owner = claim.Selector.Owner
	}

	switch {
	case !donate && !adopt:
		groupResource = "read " + groupResource
	case donate && !adopt:
		groupResource = "create user owned " + groupResource
	case !donate && adopt:
		groupResource = "have ownership of " + groupResource
	}

	_, err = fmt.Fprintf(b, "The provider wants to %s on your cluster.", groupResource)

	if owner == kubebindv1alpha1.Consumer {
		owner = "you"
	}
	if owner == kubebindv1alpha1.Provider {
		owner = "the provider"
	}
	switch {
	case owner == "" && name == "":
		_, err = fmt.Fprintf(b, "\n")
	case owner != "" && name == "":
		_, err = fmt.Fprintf(b, " This only applies to objects which are owned by %s.\n", owner)
	case owner == "" && name != "":
		_, err = fmt.Fprintf(b, " This only applies to objects which are referenced with:\n\tname: \"%s\"\n", name)
	case owner != "" && name != "":
		_, err = fmt.Fprintf(b, " This only applies to objects which are owned by %s and to objects which are referenced with:\n	name: \"%s\"\n", owner, name)
	}

	return err

}

func writeOnConflict(b *bytes.Buffer, claim kubebindv1alpha1.PermissionClaim) error {
	var err error

	if claim.OnConflict != nil {
		switch {
		case claim.OnConflict.ProviderOverwrites && claim.OnConflict.RecreateWhenConsumerSideDeleted:
			_, err = b.WriteString("Conflicting objects will be overwritten and created objects will be recreated upon deletion.\n")
		case claim.OnConflict.ProviderOverwrites:
			_, err = b.WriteString("Conflicting objects will be overwritten and created objects will not be recreated upon deletion.\n")
		case claim.OnConflict.RecreateWhenConsumerSideDeleted:
			_, err = b.WriteString("Conflicting objects will not be overwritten and created objects will be recreated upon deletion.\n")
		default: //Do nothing
		}
	}
	return err
}

func writeUpdateClause(b *bytes.Buffer, claim kubebindv1alpha1.PermissionClaim) error {
	var err error

	if claim.Update == nil {
		return nil
	}

	if claim.Update.Fields != nil {
		owner := "the provider"
		if claim.Create != nil && claim.Create.Donate {
			owner = "the user"
		}
		_, err = fmt.Fprintf(b, "The following fields of the objects will still be able to be changed by %s:\n", owner)
	}
	if claim.Update.Preserving != nil {
		_, err = b.WriteString("The following fields of the objects will be overwritten with their initial values, if they are modified:\n")
	}

	for _, s := range append(claim.Update.Fields, claim.Update.Preserving...) {
		_, err = fmt.Fprintf(b, "\t\"%s\"\n", s)
	}

	if claim.Update.AlwaysRecreate {
		_, err = b.WriteString("Modification of said objects will by handled by deletion and recreation of said objects.\n")
	}

	return err
}

func writeRequiredAndAcceptance(b *bytes.Buffer, required bool) error {
	var err error

	if required {
		_, err = fmt.Fprint(b, "Accepting this Permission is required in order to proceed.\n")
	}
	if !required {
		_, err = fmt.Fprint(b, "Accepting this Permission is optional.\n")
	}
	if err != nil {
		return nil
	}

	_, err = fmt.Fprint(b, "Do you accept this Permission? [No,Yes]\n")

	return err
}

func (opt BindAPIServiceOptions) promptYesNo(p kubebindv1alpha1.PermissionClaim) (bool, error) {

	reader := bufio.NewReader(opt.Options.IOStreams.In)

	for {
		if err := printPermissionClaim(opt.Options.Out, p); err != nil {
			return false, err
		}

		response, err := reader.ReadString('\n')
		if err != nil {
			return false, err
		}

		response = strings.ToLower(strings.TrimSpace(response))
		if response == "y" || response == "yes" {
			return true, nil
		} else if response == "n" || response == "no" {
			return false, nil
		}
	}
}
