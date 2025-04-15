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
	"context"
	"encoding/json"
	"fmt"

	"github.com/kcp-dev/logicalcluster/v3"

	kcptesting "github.com/kcp-dev/client-go/third_party/k8s.io/client-go/testing"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/testing"

	kubebindv1alpha2 "github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2"
	applyconfigurationskubebindv1alpha2 "github.com/kube-bind/kube-bind/sdk/kcp/applyconfiguration/kubebind/v1alpha2"
	kubebindv1alpha2client "github.com/kube-bind/kube-bind/sdk/kcp/clientset/versioned/typed/kubebind/v1alpha2"
)

var aPIConversionsResource = schema.GroupVersionResource{Group: "kube-bind.io", Version: "v1alpha2", Resource: "apiconversions"}
var aPIConversionsKind = schema.GroupVersionKind{Group: "kube-bind.io", Version: "v1alpha2", Kind: "APIConversion"}

type aPIConversionsClusterClient struct {
	*kcptesting.Fake
}

// Cluster scopes the client down to a particular cluster.
func (c *aPIConversionsClusterClient) Cluster(clusterPath logicalcluster.Path) kubebindv1alpha2client.APIConversionInterface {
	if clusterPath == logicalcluster.Wildcard {
		panic("A specific cluster must be provided when scoping, not the wildcard.")
	}

	return &aPIConversionsClient{Fake: c.Fake, ClusterPath: clusterPath}
}

// List takes label and field selectors, and returns the list of APIConversions that match those selectors across all clusters.
func (c *aPIConversionsClusterClient) List(ctx context.Context, opts metav1.ListOptions) (*kubebindv1alpha2.APIConversionList, error) {
	obj, err := c.Fake.Invokes(kcptesting.NewRootListAction(aPIConversionsResource, aPIConversionsKind, logicalcluster.Wildcard, opts), &kubebindv1alpha2.APIConversionList{})
	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &kubebindv1alpha2.APIConversionList{ListMeta: obj.(*kubebindv1alpha2.APIConversionList).ListMeta}
	for _, item := range obj.(*kubebindv1alpha2.APIConversionList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested APIConversions across all clusters.
func (c *aPIConversionsClusterClient) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	return c.Fake.InvokesWatch(kcptesting.NewRootWatchAction(aPIConversionsResource, logicalcluster.Wildcard, opts))
}

type aPIConversionsClient struct {
	*kcptesting.Fake
	ClusterPath logicalcluster.Path
}

func (c *aPIConversionsClient) Create(ctx context.Context, aPIConversion *kubebindv1alpha2.APIConversion, opts metav1.CreateOptions) (*kubebindv1alpha2.APIConversion, error) {
	obj, err := c.Fake.Invokes(kcptesting.NewRootCreateAction(aPIConversionsResource, c.ClusterPath, aPIConversion), &kubebindv1alpha2.APIConversion{})
	if obj == nil {
		return nil, err
	}
	return obj.(*kubebindv1alpha2.APIConversion), err
}

func (c *aPIConversionsClient) Update(ctx context.Context, aPIConversion *kubebindv1alpha2.APIConversion, opts metav1.UpdateOptions) (*kubebindv1alpha2.APIConversion, error) {
	obj, err := c.Fake.Invokes(kcptesting.NewRootUpdateAction(aPIConversionsResource, c.ClusterPath, aPIConversion), &kubebindv1alpha2.APIConversion{})
	if obj == nil {
		return nil, err
	}
	return obj.(*kubebindv1alpha2.APIConversion), err
}

func (c *aPIConversionsClient) UpdateStatus(ctx context.Context, aPIConversion *kubebindv1alpha2.APIConversion, opts metav1.UpdateOptions) (*kubebindv1alpha2.APIConversion, error) {
	obj, err := c.Fake.Invokes(kcptesting.NewRootUpdateSubresourceAction(aPIConversionsResource, c.ClusterPath, "status", aPIConversion), &kubebindv1alpha2.APIConversion{})
	if obj == nil {
		return nil, err
	}
	return obj.(*kubebindv1alpha2.APIConversion), err
}

func (c *aPIConversionsClient) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	_, err := c.Fake.Invokes(kcptesting.NewRootDeleteActionWithOptions(aPIConversionsResource, c.ClusterPath, name, opts), &kubebindv1alpha2.APIConversion{})
	return err
}

func (c *aPIConversionsClient) DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	action := kcptesting.NewRootDeleteCollectionAction(aPIConversionsResource, c.ClusterPath, listOpts)

	_, err := c.Fake.Invokes(action, &kubebindv1alpha2.APIConversionList{})
	return err
}

func (c *aPIConversionsClient) Get(ctx context.Context, name string, options metav1.GetOptions) (*kubebindv1alpha2.APIConversion, error) {
	obj, err := c.Fake.Invokes(kcptesting.NewRootGetAction(aPIConversionsResource, c.ClusterPath, name), &kubebindv1alpha2.APIConversion{})
	if obj == nil {
		return nil, err
	}
	return obj.(*kubebindv1alpha2.APIConversion), err
}

// List takes label and field selectors, and returns the list of APIConversions that match those selectors.
func (c *aPIConversionsClient) List(ctx context.Context, opts metav1.ListOptions) (*kubebindv1alpha2.APIConversionList, error) {
	obj, err := c.Fake.Invokes(kcptesting.NewRootListAction(aPIConversionsResource, aPIConversionsKind, c.ClusterPath, opts), &kubebindv1alpha2.APIConversionList{})
	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &kubebindv1alpha2.APIConversionList{ListMeta: obj.(*kubebindv1alpha2.APIConversionList).ListMeta}
	for _, item := range obj.(*kubebindv1alpha2.APIConversionList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

func (c *aPIConversionsClient) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	return c.Fake.InvokesWatch(kcptesting.NewRootWatchAction(aPIConversionsResource, c.ClusterPath, opts))
}

func (c *aPIConversionsClient) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (*kubebindv1alpha2.APIConversion, error) {
	obj, err := c.Fake.Invokes(kcptesting.NewRootPatchSubresourceAction(aPIConversionsResource, c.ClusterPath, name, pt, data, subresources...), &kubebindv1alpha2.APIConversion{})
	if obj == nil {
		return nil, err
	}
	return obj.(*kubebindv1alpha2.APIConversion), err
}

func (c *aPIConversionsClient) Apply(ctx context.Context, applyConfiguration *applyconfigurationskubebindv1alpha2.APIConversionApplyConfiguration, opts metav1.ApplyOptions) (*kubebindv1alpha2.APIConversion, error) {
	if applyConfiguration == nil {
		return nil, fmt.Errorf("applyConfiguration provided to Apply must not be nil")
	}
	data, err := json.Marshal(applyConfiguration)
	if err != nil {
		return nil, err
	}
	name := applyConfiguration.Name
	if name == nil {
		return nil, fmt.Errorf("applyConfiguration.Name must be provided to Apply")
	}
	obj, err := c.Fake.Invokes(kcptesting.NewRootPatchSubresourceAction(aPIConversionsResource, c.ClusterPath, *name, types.ApplyPatchType, data), &kubebindv1alpha2.APIConversion{})
	if obj == nil {
		return nil, err
	}
	return obj.(*kubebindv1alpha2.APIConversion), err
}

func (c *aPIConversionsClient) ApplyStatus(ctx context.Context, applyConfiguration *applyconfigurationskubebindv1alpha2.APIConversionApplyConfiguration, opts metav1.ApplyOptions) (*kubebindv1alpha2.APIConversion, error) {
	if applyConfiguration == nil {
		return nil, fmt.Errorf("applyConfiguration provided to Apply must not be nil")
	}
	data, err := json.Marshal(applyConfiguration)
	if err != nil {
		return nil, err
	}
	name := applyConfiguration.Name
	if name == nil {
		return nil, fmt.Errorf("applyConfiguration.Name must be provided to Apply")
	}
	obj, err := c.Fake.Invokes(kcptesting.NewRootPatchSubresourceAction(aPIConversionsResource, c.ClusterPath, *name, types.ApplyPatchType, data, "status"), &kubebindv1alpha2.APIConversion{})
	if obj == nil {
		return nil, err
	}
	return obj.(*kubebindv1alpha2.APIConversion), err
}
