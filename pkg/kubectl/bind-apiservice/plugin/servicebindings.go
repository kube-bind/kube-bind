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

	if err := writeCreate(&b, p); err != nil {
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

	donate := claim.AutoDonate

	adopt := claim.AutoAdopt

	var names []string
	var owner kubebindv1alpha1.PermissionCaimResourceOwner
	if claim.Selector != nil {
		names = claim.Selector.Names
		owner = claim.Selector.Owner
	}

	var verb string
	switch owner {
	case kubebindv1alpha1.Provider:
		verb = "write"
	case kubebindv1alpha1.Consumer:
		verb = "read"
	default:
		verb = "read and write"
	}

	switch {
	case !donate && !adopt:
		groupResource = verb + " " + groupResource
	case donate && !adopt:
		groupResource = "create user owned " + groupResource
	case !donate && adopt:
		groupResource = "have ownership of " + groupResource
	}

	var ref string
	if len(names) > 0 {
		ref = " which are referenced with:"
		for _, name := range names {
			ref = fmt.Sprintf("%s\n\t- name: \"%s\"", ref, name)
		}
		ref += "\n"
	} else {
		ref += " "
	}

	_, err = fmt.Fprintf(b, "The provider wants to %s%son your cluster.\n", groupResource, ref)

	return err

}

func writeCreate(b io.StringWriter, claim kubebindv1alpha1.PermissionClaim) error {
	var err error

	switch {
	case claim.Create == nil || !claim.Create.ReplaceExisting:
		//_, err = b.WriteString("Conflicting objects will not be overwritten. ")
	case claim.Create.ReplaceExisting:
		_, err = b.WriteString("Conflicting objects will be replaced by the provider. ")
	}

	return err
}

func writeOnConflict(b io.StringWriter, claim kubebindv1alpha1.PermissionClaim) error {
	var err error

	switch {
	case claim.OnConflict == nil || !claim.OnConflict.RecreateWhenConsumerSideDeleted:
		//_, err = b.WriteString("Created objects will not be recreated upon deletion. ")
	case claim.OnConflict.RecreateWhenConsumerSideDeleted:
		_, err = b.WriteString("Created objects will be recreated upon deletion. ")
	default: //Do nothing
	}

	return err
}

func writeUpdateClause(b *bytes.Buffer, claim kubebindv1alpha1.PermissionClaim) error {
	var err error

	if claim.Update == nil {
		return nil
	}

	if claim.Update.Fields != nil {
		_, err = fmt.Fprintf(b, "The following fields of the objects will still be able to be changed by the provider:\n")
	}
	if claim.Update.Preserving != nil {
		_, err = b.WriteString("The following fields of the objects will be preserved by the provider:\n")
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
