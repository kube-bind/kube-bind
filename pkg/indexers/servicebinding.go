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

package indexers

import (
	"fmt"

	kubebindv1alpha2 "github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2"
)

const (
	//nolint:gosec
	ByServiceBindingKubeconfigSecret = "byKubeconfigSecret"
	ByAPIServiceBindingCRD           = "byCRD"
)

func IndexServiceBindingByKubeconfigSecret(obj any) ([]string, error) {
	binding, ok := obj.(*kubebindv1alpha2.APIServiceBinding)
	if !ok {
		return nil, nil
	}
	return []string{ByServiceBindingKubeconfigSecretKey(binding)}, nil
}

func ByServiceBindingKubeconfigSecretKey(binding *kubebindv1alpha2.APIServiceBinding) string {
	ref := &binding.Spec.KubeconfigSecretRef
	return ref.Namespace + "/" + ref.Name
}

func IndexAPIServiceBindingByCRD(obj any) ([]string, error) {
	binding, ok := obj.(*kubebindv1alpha2.APIServiceBinding)
	if !ok {
		return nil, nil
	}

	keys := make([]string, 0, len(binding.Status.BoundSchemas))
	for _, bound := range binding.Status.BoundSchemas {
		keys = append(keys, fmt.Sprintf("%s.%s", bound.Resource, bound.Group))
	}

	return keys, nil
}
