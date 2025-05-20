package indexers

import (
	"strings"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	kubebindv1alpha2 "github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2"
)

const (
	CRDByServiceBinding = "CRDByServiceBinding"
)

func IndexCRDByServiceBinding(obj any) ([]string, error) {
	crd, ok := obj.(*apiextensionsv1.CustomResourceDefinition)
	if !ok {
		return nil, nil
	}

	bindings := []string{}
	for _, ref := range crd.OwnerReferences {
		parts := strings.SplitN(ref.APIVersion, "/", 2)
		if parts[0] != kubebindv1alpha2.SchemeGroupVersion.Group || ref.Kind != "APIServiceBinding" {
			continue
		}
		bindings = append(bindings, ref.Name)
	}
	return bindings, nil
}
