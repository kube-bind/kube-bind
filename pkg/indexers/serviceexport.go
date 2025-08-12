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
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2"
)

const (
	ServiceExportByCustomResourceDefinition = "serviceExportByCustomResourceDefinition"
)

func IndexServiceExportByCustomResourceDefinition(obj any) ([]string, error) {
	export, ok := obj.(*v1alpha2.APIServiceExport)
	if !ok {
		return nil, nil
	}

	return []string{export.Name}, nil
}

// IndexServiceExportByCustomResourceDefinitionControllerRuntime is a controller-runtime compatible indexer function
// that indexes APIServiceExports by their CustomResourceDefinition name.
func IndexServiceExportByCustomResourceDefinitionControllerRuntime(obj client.Object) []string {
	export, ok := obj.(*v1alpha2.APIServiceExport)
	if !ok {
		return nil
	}

	names := make([]string, 0, len(export.Spec.Resources))
	for _, resource := range export.Spec.Resources {
		names = append(names, resource.Name)
	}

	return names
}
