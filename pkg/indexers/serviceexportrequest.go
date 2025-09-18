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

	kubebindv1alpha2 "github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2"
)

const (
	ServiceExportRequestByGroupResource = "ServiceExportRequestByGroupResource"
	ServiceExportRequestByServiceExport = "ServiceExportRequestByServiceExport"
)

// IndexServiceExportRequestByGroupResource is a controller-runtime compatible indexer function
// that indexes APIServiceExportRequests by their Group.Resource name.
func IndexServiceExportRequestByGroupResource(obj client.Object) []string {
	apiServiceExportRequest, ok := obj.(*kubebindv1alpha2.APIServiceExportRequest)
	if !ok {
		return nil
	}
	keys := []string{}
	for _, gr := range apiServiceExportRequest.Spec.Resources {
		keys = append(keys, gr.ResourceGroupName())
	}
	return keys
}

// IndexServiceExportRequestByServiceExport is a controller-runtime compatible indexer function
// that indexes APIServiceExportRequests by their related ServiceExport name.
func IndexServiceExportRequestByServiceExport(obj client.Object) []string {
	apiServiceExportRequest, ok := obj.(*kubebindv1alpha2.APIServiceExportRequest)
	if !ok {
		return nil
	}
	keys := []string{}
	for _, gr := range apiServiceExportRequest.Spec.Resources {
		keys = append(keys, apiServiceExportRequest.Namespace+"/"+gr.ResourceGroupName())
	}
	return keys
}
