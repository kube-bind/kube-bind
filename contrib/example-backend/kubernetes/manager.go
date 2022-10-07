/*
Copyright 2022 The Kubectl Bind contributors.

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

func (m *Manager) HandleResources(ctx context.Context) ([]byte, error) {
	err := kuberesources.CreateNamespace(ctx, m.namespace, m.kubeClient)
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

	kfgSecret, err := kuberesources.GenerateKubeconfig(ctx, m.kubeClient, m.clusterConfig.Host, m.clusterName, sa.Name, sa.Namespace)
	if err != nil {
		return nil, err
	}

	if err := kuberesources.CreateClusterBinding(ctx, m.bindClient, sa.Name, m.namespace); err != nil {
		return nil, err
	}

	return kfgSecret.Data["kubeconfig"], nil
}
