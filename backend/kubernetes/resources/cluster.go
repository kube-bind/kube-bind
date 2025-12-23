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

package resources

import (
	"context"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kubebindv1alpha2 "github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2"
)

func CreateDefaultCluster(ctx context.Context, c client.Client) (*kubebindv1alpha2.Cluster, error) {
	cluster := &kubebindv1alpha2.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: kubebindv1alpha2.DefaultClusterName,
		},
		Spec: kubebindv1alpha2.ClusterSpec{},
	}

	err := c.Create(ctx, cluster)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return nil, err
	}

	return cluster, nil
}
