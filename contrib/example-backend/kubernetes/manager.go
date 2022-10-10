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

package kubernetes

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeclient "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	kuberesources "github.com/kube-bind/kube-bind/contrib/example-backend/kubernetes/resources"
	bindclient "github.com/kube-bind/kube-bind/pkg/client/clientset/versioned"
)

type Manager struct {
	clusterConfig *rest.Config
	kubeClient    kubeclient.Interface
	bindClient    bindclient.Interface
	namespace     string
	clusterName   string
}

func NewKubernetesManager(config *rest.Config, clusterName, ns string) (*Manager, error) {
	kubeClient, err := kubeclient.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	err = kuberesources.CreateNamespace(context.TODO(), ns, kubeClient)
	if err != nil {
		return nil, err
	}

	bindClient, err := bindclient.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &Manager{
		kubeClient:    kubeClient,
		bindClient:    bindClient,
		clusterConfig: config,
		namespace:     ns,
		clusterName:   clusterName,
	}, nil
}

func (m *Manager) SaveUserData(ctx context.Context, auth *kuberesources.AuthCode) error {
	secret, err := m.kubeClient.CoreV1().Secrets(m.namespace).Get(ctx, kuberesources.SessionIDs, v1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			secret = &corev1.Secret{
				ObjectMeta: v1.ObjectMeta{
					Name:      kuberesources.SessionIDs,
					Namespace: m.namespace,
				},
				Data: map[string][]byte{
					auth.SessionID: []byte(auth.RedirectURL),
				},
			}

			_, err := m.kubeClient.CoreV1().Secrets(m.namespace).Create(ctx, secret, v1.CreateOptions{})
			if err == nil {
				return nil
			}
		}

		return err
	}

	secret.Data[auth.SessionID] = []byte(auth.RedirectURL)
	_, err = m.kubeClient.CoreV1().Secrets(m.namespace).Update(ctx, secret, v1.UpdateOptions{})
	return err
}

func (m *Manager) FetchUserData(ctx context.Context) (*corev1.Secret, error) {
	return m.kubeClient.CoreV1().Secrets(m.namespace).Get(ctx, kuberesources.SessionIDs, v1.GetOptions{})
}

func (m *Manager) HandleResources(ctx context.Context) ([]byte, error) {
	saSecret, err := kuberesources.CreateSASecret(ctx, m.kubeClient, m.namespace)
	if err != nil {
		return nil, err
	}

	sa, err := kuberesources.CreateServiceAccount(ctx, m.kubeClient, m.namespace)
	if err != nil {
		return nil, err
	}

	if err := kuberesources.CreateAdminClusterRoleBinding(ctx, m.kubeClient, m.namespace); err != nil {
		return nil, err
	}

	kfgSecret, err := kuberesources.GenerateKubeconfig(ctx, m.kubeClient, m.clusterConfig.Host, m.clusterName, saSecret.Name, sa.Namespace)
	if err != nil {
		return nil, err
	}

	if err := kuberesources.CreateClusterBinding(ctx, m.bindClient, sa.Name, m.namespace); err != nil {
		return nil, err
	}

	return kfgSecret.Data["kubeconfig"], nil
}
