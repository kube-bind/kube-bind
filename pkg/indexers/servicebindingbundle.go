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

package indexers

import (
	kubebindv1alpha2 "github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2"
)

const (
	//nolint:gosec
	ByServiceBindingBundleKubeconfigSecret = "byKubeconfigSecretBundle"
)

func IndexServiceBindingBundleByKubeconfigSecret(obj any) ([]string, error) {
	bundle, ok := obj.(*kubebindv1alpha2.APIServiceBindingBundle)
	if !ok {
		return nil, nil
	}
	return []string{ByServiceBindingBundleKubeconfigSecretKey(bundle)}, nil
}

func ByServiceBindingBundleKubeconfigSecretKey(bundle *kubebindv1alpha2.APIServiceBindingBundle) string {
	ref := &bundle.Spec.KubeconfigSecretRef
	return ref.Namespace + "/" + ref.Name
}
