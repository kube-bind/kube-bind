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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeclient "k8s.io/client-go/kubernetes"
)

const (
	ClusterIDAnnotationKey = "kube-bind.io/cluster-id"
)

// EnsureServiceBindingAuthData create a secret which contains the service binding authenticated data such as
// the binding session id and the kubeconfig of the service provider cluster.
func EnsureServiceBindingAuthData(ctx context.Context, kubeconfig, sessionID, clusterID, ns string, client kubeclient.Interface) (string, error) {
	kfgSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "provider-kubeconfig",
			Namespace: ns,
			Annotations: map[string]string{
				ClusterIDAnnotationKey: clusterID,
			},
		},
		Data: map[string][]byte{
			"kubeconfig": []byte(kubeconfig),
			"sessionID":  []byte(sessionID),
		},
	}

	if _, err := client.CoreV1().Secrets(ns).Create(ctx, kfgSecret, metav1.CreateOptions{}); err != nil && !errors.IsAlreadyExists(err) {
		return "", err
	}

	return kfgSecret.Name, nil
}
