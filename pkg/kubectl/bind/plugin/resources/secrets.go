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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeclient "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
)

const (
	ClusterIDAnnotationKey = "kube-bind.io/cluster-id"
)

// EnsureServiceBindingAuthData create a secret which contains the service binding authenticated data such as
// the binding session id and the kubeconfig of the service provider cluster. If it is pre-existing, the kubeconfig
// is updated.
func EnsureServiceBindingAuthData(ctx context.Context, kubeconfig, clusterID, ns, name string, client kubeclient.Interface) (string, error) {
	if name == "" {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:    ns,
				GenerateName: "kubeconfig-",
				Annotations: map[string]string{
					ClusterIDAnnotationKey: clusterID,
				},
			},
			Data: map[string][]byte{
				"kubeconfig": []byte(kubeconfig),
			},
		}

		secret, err := client.CoreV1().Secrets(ns).Create(ctx, secret, metav1.CreateOptions{})
		if err != nil {
			return "", err
		}
		fmt.Printf("Created secret %s/%s\n", ns, secret.Name)
		return secret.Name, nil
	}

	// update existing secret
	var secret *corev1.Secret
	fmt.Printf("Updating secret %s/%s\n", ns, name)
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var err error
		secret, err = client.CoreV1().Secrets(ns).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if secret.Annotations[ClusterIDAnnotationKey] != clusterID {
			return errors.NewAlreadyExists(corev1.Resource("secret"), secret.Name)
		}
		secret.Data["kubeconfig"] = []byte(kubeconfig)
		if _, err := client.CoreV1().Secrets(ns).Update(ctx, secret, metav1.UpdateOptions{}); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return "", err
	}

	return secret.Name, nil
}
