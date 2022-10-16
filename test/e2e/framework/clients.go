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

package framework

import (
	"testing"

	"github.com/stretchr/testify/require"

	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func DynamicClient(t *testing.T, config *rest.Config) dynamic.Interface {
	c, err := dynamic.NewForConfig(config)
	require.NoError(t, err)
	return c
}

func KubeClient(t *testing.T, config *rest.Config) kubernetes.Interface {
	c, err := kubernetes.NewForConfig(config)
	require.NoError(t, err)
	return c
}

func ApiextensionsClient(t *testing.T, config *rest.Config) apiextensionsclient.Interface {
	c, err := apiextensionsclient.NewForConfig(config)
	require.NoError(t, err)
	return c
}

func DiscoveryClient(t *testing.T, config *rest.Config) discovery.DiscoveryInterface {
	c, err := discovery.NewDiscoveryClientForConfig(config)
	require.NoError(t, err)
	return c
}
