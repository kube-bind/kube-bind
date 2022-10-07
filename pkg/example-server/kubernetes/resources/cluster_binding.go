/*
Copyright 2022 The Kubectl Bind API contributors.

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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"

	"github.com/kube-bind/kube-bind/pkg/apis/kubebindapi/v1alpha1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func CreateClusterBinding(ctx context.Context, config *rest.Config, name, ns string) error {
	clusterBinding := &v1alpha1.ClusterBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: v1alpha1.ClusterBindingSpec{
			KubeconfigSecretRef: corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: name,
				},
				Key: "kubeconfig",
			},
		},
	}

	restClient, err := rest.RESTClientFor(config)
	if err != nil {
		return err
	}

	return restClient.
		Post().
		Namespace(ns).
		Resource("clusterbindings").
		Body(clusterBinding).
		Do(ctx).Error()
}
