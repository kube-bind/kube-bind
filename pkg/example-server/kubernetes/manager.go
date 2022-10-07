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

package kubernetes

import (
	"context"
	"github.com/kube-bind/kube-bind/pkg/example-server/kubernetes/resources"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type Manager struct {
	clusterConfig *rest.Config
	client        *k8s.Clientset
	namespace     string
	clusterName   string
}

func NewKubernetesManager(config *rest.Config, clusterName, ns string) (*Manager, error) {
	client, err := k8s.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &Manager{
		client:        client,
		clusterConfig: config,
		namespace:     ns,
		clusterName:   clusterName,
	}, nil
}

func (m *Manager) HandleResources(ctx context.Context) ([]byte, error) {
	err := resources.CreateNamespace(ctx, m.namespace, m.client)
	if err != nil {
		return nil, err
	}

	sa, err := resources.CreateServiceAccount(ctx, m.client, m.namespace)
	if err != nil {
		return nil, err
	}

	if err := resources.CreateAdminClusterRoleBinding(ctx, m.client, m.namespace); err != nil {
		return nil, err
	}

	kfgSecret, err := resources.GenerateKubeconfig(ctx, m.client, m.clusterConfig.Host, m.clusterName, sa.Name, sa.Namespace)
	if err != nil {
		return nil, err
	}

	if err := resources.CreateClusterBinding(ctx, m.clusterConfig, sa.Name, m.namespace); err != nil {
		return nil, err
	}

	return kfgSecret.Data["kubeconfig"], nil
}
