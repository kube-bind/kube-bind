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
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

func GenerateKubeconfig(ctx context.Context,
	client kubernetes.Interface,
	host, ns, secretName string,
) (*corev1.Secret, error) {
	kfg, err := client.CoreV1().Secrets(ns).Get(ctx, ClusterBindingKubeConfig, v1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			var secret *corev1.Secret
			if err := wait.PollImmediateWithContext(ctx, 500*time.Millisecond, 10*time.Second, func(ctx context.Context) (done bool, err error) {
				secret, err = client.CoreV1().Secrets(ns).Get(ctx, secretName, v1.GetOptions{})
				if err != nil && !errors.IsNotFound(err) {
					return false, err
				} else if errors.IsNotFound(err) {
					return false, nil
				}
				return secret.Data["token"] != nil && secret.Data["ca.crt"] != nil, nil
			}); err != nil {
				return nil, err
			}

			cfg := api.Config{
				Clusters: map[string]*api.Cluster{
					"provider": {
						Server:                   host,
						CertificateAuthorityData: secret.Data["ca.crt"],
					},
				},
				Contexts: map[string]*api.Context{
					"default": {
						Cluster:   "provider",
						Namespace: ns,
						AuthInfo:  "default",
					},
				},
				AuthInfos: map[string]*api.AuthInfo{
					"default": {
						Token: string(secret.Data["token"]),
					},
				},
				CurrentContext: "default",
			}

			kubeconfig, err := clientcmd.Write(cfg)
			if err != nil {
				return nil, fmt.Errorf("failed to encode kubeconfig: %w", err)
			}

			kfg = &corev1.Secret{
				ObjectMeta: v1.ObjectMeta{
					Name:      "cluster-admin-kubeconfig",
					Namespace: ns,
				},
			}

			kfg.Data = map[string][]byte{}
			kfg.Data["kubeconfig"] = kubeconfig

			return client.CoreV1().Secrets(ns).Create(ctx, kfg, v1.CreateOptions{})
		}

		return nil, err
	}

	return kfg, nil
}
