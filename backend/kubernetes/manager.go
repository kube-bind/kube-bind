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
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	mcmanager "sigs.k8s.io/multicluster-runtime/pkg/manager"

	kuberesources "github.com/kube-bind/kube-bind/backend/kubernetes/resources"
	kubebindv1alpha2 "github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2"
)

type Manager struct {
	namespacePrefix    string
	providerPrettyName string

	externalAddressGenerator kuberesources.ExternalAddreesGeneratorFunc
	externalCA               []byte
	externalTLSServerName    string

	manager mcmanager.Manager
}

func NewKubernetesManager(
	ctx context.Context,
	namespacePrefix, providerPrettyName string,
	externalAddressGenerator kuberesources.ExternalAddreesGeneratorFunc,
	externalCA []byte,
	externalTLSServerName string,
	manager mcmanager.Manager,
) (*Manager, error) {
	m := &Manager{
		namespacePrefix:    namespacePrefix,
		providerPrettyName: providerPrettyName,

		externalAddressGenerator: externalAddressGenerator,
		externalCA:               externalCA,
		externalTLSServerName:    externalTLSServerName,

		manager: manager,
	}

	if err := m.manager.GetFieldIndexer().IndexField(ctx, &corev1.Namespace{}, NamespacesByIdentity,
		IndexNamespacesByIdentity); err != nil {
		return nil, fmt.Errorf("failed to setup NamespacesByIdentity indexer: %w", err)
	}

	return m, nil
}

func (m *Manager) HandleResources(ctx context.Context, identity, cluster, resource, group string) ([]byte, error) {
	logger := klog.FromContext(ctx).WithValues("identity", identity, "resource", resource, "group", group)
	ctx = klog.NewContext(ctx, logger)

	cl, err := m.manager.GetCluster(ctx, cluster)
	if err != nil {
		return nil, err
	}
	c := cl.GetClient()

	// try to find an existing namespace by annotation, or create a new one.
	var nss corev1.NamespaceList
	err = c.List(ctx, &nss, client.MatchingFields{NamespacesByIdentity: identity})
	if err != nil {
		return nil, err
	}
	if len(nss.Items) > 1 {
		logger.Error(fmt.Errorf("found multiple namespaces for identity %q", identity), "found multiple namespaces for identity")
		return nil, fmt.Errorf("found multiple namespaces for identity %q", identity)
	}
	var ns string
	if len(nss.Items) == 1 {
		ns = nss.Items[0].Name
	} else {
		nsObj, err := kuberesources.CreateNamespace(ctx, c, m.namespacePrefix, identity)
		if err != nil {
			return nil, err
		}
		logger.Info("Created namespace", "namespace", nsObj.Name)
		ns = nsObj.Name
	}
	logger = logger.WithValues("namespace", ns)
	ctx = klog.NewContext(ctx, logger)

	// first look for ClusterBinding to get old secret name
	kubeconfigSecretName := kuberesources.KubeconfigSecretName
	var cb kubebindv1alpha2.ClusterBinding
	err = c.Get(ctx, types.NamespacedName{Namespace: ns, Name: kuberesources.ClusterBindingName}, &cb)
	switch {
	case errors.IsNotFound(err):
		if err := kuberesources.CreateClusterBinding(ctx, c, ns, "kubeconfig", m.providerPrettyName); err != nil {
			return nil, err
		}
	case err != nil:
		return nil, err
	default:
		logger.V(3).Info("Found existing ClusterBinding")
		kubeconfigSecretName = cb.Spec.KubeconfigSecretRef.Name // reuse old name
	}

	sa, err := kuberesources.CreateServiceAccount(ctx, c, ns, kuberesources.ServiceAccountName)
	if err != nil {
		return nil, err
	}

	if err := kuberesources.EnsureBinderClusterRole(ctx, c); err != nil {
		return nil, err
	}

	saSecret, err := kuberesources.CreateSASecret(ctx, c, ns, sa.Name)
	if err != nil {
		return nil, err
	}

	kfgSecret, err := kuberesources.GenerateKubeconfig(ctx, c, cl.GetConfig(), m.externalAddressGenerator, m.externalCA, m.externalTLSServerName, saSecret.Name, ns, kubeconfigSecretName)
	if err != nil {
		return nil, err
	}

	return kfgSecret.Data["kubeconfig"], nil
}

func (m *Manager) ListAPIResourceSchemas(ctx context.Context, cluster string) (*kubebindv1alpha2.APIResourceSchemaList, error) {
	cl, err := m.manager.GetCluster(ctx, cluster)
	if err != nil {
		return nil, err
	}
	cache := cl.GetCache()

	var schemas kubebindv1alpha2.APIResourceSchemaList
	err = cache.List(ctx, &schemas)
	if err != nil {
		return nil, err
	}
	return &schemas, nil
}

func (m *Manager) ListCustomResourceDefinitions(ctx context.Context, cluster string, selector labels.Selector) (*apiextensionsv1.CustomResourceDefinitionList, error) {
	cl, err := m.manager.GetCluster(ctx, cluster)
	if err != nil {
		return nil, err
	}
	c := cl.GetClient()

	var crds apiextensionsv1.CustomResourceDefinitionList
	err = c.List(ctx, &crds, client.MatchingLabelsSelector{Selector: selector})
	if err != nil {
		return nil, err
	}

	return &crds, nil
}
