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

package resources

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	kubebindv1alpha1 "github.com/kube-bind/kube-bind/pkg/apis/kubebind/v1alpha1"
	bindclient "github.com/kube-bind/kube-bind/pkg/client/clientset/versioned"
	"github.com/kube-bind/kube-bind/pkg/indexers"
)

func CreateAPIServiceExport(ctx context.Context, client bindclient.Interface, serviceExport cache.Indexer, ns, resource, group string) error {
	logging := klog.FromContext(ctx)

	exports, err := serviceExport.ByIndex(indexers.ServiceExportByServiceExportResource, indexers.ServiceExportByServiceExportResourceKey(ns, resource, group))
	if err != nil {
		return fmt.Errorf("failed to get service export for resource %s.%s: %w", resource, group, err)
	}

	if len(exports) > 0 {
		logging.Info("Service export already exists", "name", resource+"."+group)
		return nil
	}

	logging.Info("Creating service export", "name", resource+"."+group)
	_, err = client.KubeBindV1alpha1().APIServiceExports(ns).Create(ctx, &kubebindv1alpha1.APIServiceExport{
		ObjectMeta: metav1.ObjectMeta{
			Name:      resource + "." + group,
			Namespace: ns,
		},
		Spec: kubebindv1alpha1.APIServiceExportSpec{
			Scope: kubebindv1alpha1.ClusterScope,
			Resources: []kubebindv1alpha1.APIServiceExportGroupResource{
				{
					GroupResource: kubebindv1alpha1.GroupResource{
						Group:    group,
						Resource: resource,
					},
				},
			},
		},
	}, metav1.CreateOptions{})
	return err
}
