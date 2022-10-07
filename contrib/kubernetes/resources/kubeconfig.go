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
	"fmt"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

func GenerateKubeconfig(ctx context.Context,
	client *kubernetes.Clientset,
	host, clusterName, saName, ns string) (*corev1.Secret, error) {
	kfg, err := client.CoreV1().Secrets(ns).Get(ctx, "cluster-admin-kubeconfig", v1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			sa, err := client.CoreV1().ServiceAccounts(ns).Get(ctx, saName, v1.GetOptions{})
			if err != nil {
				return nil, err
			}

			if len(sa.Secrets) >= 1 {
				secret, err := client.CoreV1().Secrets(ns).Get(ctx, sa.Secrets[0].Name, v1.GetOptions{})
				if err != nil {
					return nil, err
				}

				cfg := api.Config{}
				cfg.Clusters = map[string]*api.Cluster{
					"": {
						Server:                   host,
						CertificateAuthorityData: secret.Data["ca.crt"],
					},
				}

				cfg.Contexts = map[string]*api.Context{
					"default": {
						Cluster:   clusterName,
						Namespace: ns,
						AuthInfo:  "default",
					},
				}
				cfg.CurrentContext = "default"
				cfg.AuthInfos = map[string]*api.AuthInfo{
					"default": {
						Token: string(secret.Data["token"]),
					},
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

			return nil, fmt.Errorf("failed to find any secrets belong to service name %q", saName)
		}

		return nil, err
	}

	return kfg, nil
}
