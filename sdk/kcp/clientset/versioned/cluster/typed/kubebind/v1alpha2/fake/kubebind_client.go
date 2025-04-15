//go:build !ignore_autogenerated
// +build !ignore_autogenerated

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

// Code generated by kcp code-generator. DO NOT EDIT.

package fake

import (
	"github.com/kcp-dev/logicalcluster/v3"

	kcptesting "github.com/kcp-dev/client-go/third_party/k8s.io/client-go/testing"
	"k8s.io/client-go/rest"

	kcpkubebindv1alpha2 "github.com/kube-bind/kube-bind/sdk/kcp/clientset/versioned/cluster/typed/kubebind/v1alpha2"
	kubebindv1alpha2 "github.com/kube-bind/kube-bind/sdk/kcp/clientset/versioned/typed/kubebind/v1alpha2"
)

var _ kcpkubebindv1alpha2.KubeBindV1alpha2ClusterInterface = (*KubeBindV1alpha2ClusterClient)(nil)

type KubeBindV1alpha2ClusterClient struct {
	*kcptesting.Fake
}

func (c *KubeBindV1alpha2ClusterClient) Cluster(clusterPath logicalcluster.Path) kubebindv1alpha2.KubeBindV1alpha2Interface {
	if clusterPath == logicalcluster.Wildcard {
		panic("A specific cluster must be provided when scoping, not the wildcard.")
	}
	return &KubeBindV1alpha2Client{Fake: c.Fake, ClusterPath: clusterPath}
}

func (c *KubeBindV1alpha2ClusterClient) APIResourceSchemas() kcpkubebindv1alpha2.APIResourceSchemaClusterInterface {
	return &aPIResourceSchemasClusterClient{Fake: c.Fake}
}

func (c *KubeBindV1alpha2ClusterClient) APIConversions() kcpkubebindv1alpha2.APIConversionClusterInterface {
	return &aPIConversionsClusterClient{Fake: c.Fake}
}

var _ kubebindv1alpha2.KubeBindV1alpha2Interface = (*KubeBindV1alpha2Client)(nil)

type KubeBindV1alpha2Client struct {
	*kcptesting.Fake
	ClusterPath logicalcluster.Path
}

func (c *KubeBindV1alpha2Client) RESTClient() rest.Interface {
	var ret *rest.RESTClient
	return ret
}

func (c *KubeBindV1alpha2Client) APIResourceSchemas(namespace string) kubebindv1alpha2.APIResourceSchemaInterface {
	return &aPIResourceSchemasClient{Fake: c.Fake, ClusterPath: c.ClusterPath, Namespace: namespace}
}

func (c *KubeBindV1alpha2Client) APIConversions() kubebindv1alpha2.APIConversionInterface {
	return &aPIConversionsClient{Fake: c.Fake, ClusterPath: c.ClusterPath}
}
