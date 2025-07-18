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

// Code generated by client-gen-v0.31. DO NOT EDIT.

package fake

import (
	rest "k8s.io/client-go/rest"
	testing "k8s.io/client-go/testing"

	v1alpha2 "github.com/kube-bind/kube-bind/sdk/kcp/clientset/versioned/typed/kubebind/v1alpha2"
)

type FakeKubeBindV1alpha2 struct {
	*testing.Fake
}

func (c *FakeKubeBindV1alpha2) APIResourceSchemas() v1alpha2.APIResourceSchemaInterface {
	return &FakeAPIResourceSchemas{c}
}

func (c *FakeKubeBindV1alpha2) APIServiceBindings() v1alpha2.APIServiceBindingInterface {
	return &FakeAPIServiceBindings{c}
}

func (c *FakeKubeBindV1alpha2) APIServiceExports(namespace string) v1alpha2.APIServiceExportInterface {
	return &FakeAPIServiceExports{c, namespace}
}

func (c *FakeKubeBindV1alpha2) APIServiceExportRequests(namespace string) v1alpha2.APIServiceExportRequestInterface {
	return &FakeAPIServiceExportRequests{c, namespace}
}

func (c *FakeKubeBindV1alpha2) APIServiceNamespaces(namespace string) v1alpha2.APIServiceNamespaceInterface {
	return &FakeAPIServiceNamespaces{c, namespace}
}

func (c *FakeKubeBindV1alpha2) BoundAPIResourceSchemas(namespace string) v1alpha2.BoundAPIResourceSchemaInterface {
	return &FakeBoundAPIResourceSchemas{c, namespace}
}

func (c *FakeKubeBindV1alpha2) ClusterBindings(namespace string) v1alpha2.ClusterBindingInterface {
	return &FakeClusterBindings{c, namespace}
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *FakeKubeBindV1alpha2) RESTClient() rest.Interface {
	var ret *rest.RESTClient
	return ret
}
