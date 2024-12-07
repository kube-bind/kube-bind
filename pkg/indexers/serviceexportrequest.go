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
	kubebindv1alpha1 "github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha1"
)

const (
	ServiceExportRequestByGroupResource = "ServiceExportRequestByGroupResource"
	ServiceExportRequestByServiceExport = "ServiceExportRequestByServiceExport"
)

func IndexServiceExportRequestByGroupResource(obj interface{}) ([]string, error) {
	sbr, ok := obj.(*kubebindv1alpha1.APIServiceExportRequest)
	if !ok {
		return nil, nil
	}
	keys := []string{}
	for _, gr := range sbr.Spec.Resources {
		keys = append(keys, gr.GroupResource.Resource+"."+gr.GroupResource.Group)
	}
	return keys, nil
}

func IndexServiceExportRequestByServiceExport(obj interface{}) ([]string, error) {
	sbr, ok := obj.(*kubebindv1alpha1.APIServiceExportRequest)
	if !ok {
		return nil, nil
	}
	keys := []string{}
	for _, gr := range sbr.Spec.Resources {
		keys = append(keys, sbr.Namespace+"/"+gr.GroupResource.Resource+"."+gr.GroupResource.Group)
	}
	return keys, nil
}
