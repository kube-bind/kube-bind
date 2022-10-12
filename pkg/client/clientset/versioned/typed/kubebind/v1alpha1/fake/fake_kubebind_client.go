/*
Copyright The Kube Bind Authors.

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

// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	rest "k8s.io/client-go/rest"
	testing "k8s.io/client-go/testing"

	v1alpha1 "github.com/kube-bind/kube-bind/pkg/client/clientset/versioned/typed/kubebind/v1alpha1"
)

type FakeKubeBindV1alpha1 struct {
	*testing.Fake
}

func (c *FakeKubeBindV1alpha1) ClusterBindings(namespace string) v1alpha1.ClusterBindingInterface {
	return &FakeClusterBindings{c, namespace}
}

func (c *FakeKubeBindV1alpha1) ServiceBindings() v1alpha1.ServiceBindingInterface {
	return &FakeServiceBindings{c}
}

func (c *FakeKubeBindV1alpha1) ServiceBindingSessions(namespace string) v1alpha1.ServiceBindingSessionInterface {
	return &FakeServiceBindingSessions{c, namespace}
}

func (c *FakeKubeBindV1alpha1) ServiceExports(namespace string) v1alpha1.ServiceExportInterface {
	return &FakeServiceExports{c, namespace}
}

func (c *FakeKubeBindV1alpha1) ServiceExportResources(namespace string) v1alpha1.ServiceExportResourceInterface {
	return &FakeServiceExportResources{c, namespace}
}

func (c *FakeKubeBindV1alpha1) ServiceNamespaces(namespace string) v1alpha1.ServiceNamespaceInterface {
	return &FakeServiceNamespaces{c, namespace}
}

func (c *FakeKubeBindV1alpha1) ServiceProviders(namespace string) v1alpha1.ServiceProviderInterface {
	return &FakeServiceProviders{c, namespace}
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *FakeKubeBindV1alpha1) RESTClient() rest.Interface {
	var ret *rest.RESTClient
	return ret
}
