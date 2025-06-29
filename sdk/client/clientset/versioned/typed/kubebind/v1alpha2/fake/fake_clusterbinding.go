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
	"context"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"

	v1alpha2 "github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2"
)

// FakeClusterBindings implements ClusterBindingInterface
type FakeClusterBindings struct {
	Fake *FakeKubeBindV1alpha2
	ns   string
}

var clusterbindingsResource = v1alpha2.SchemeGroupVersion.WithResource("clusterbindings")

var clusterbindingsKind = v1alpha2.SchemeGroupVersion.WithKind("ClusterBinding")

// Get takes name of the clusterBinding, and returns the corresponding clusterBinding object, and an error if there is any.
func (c *FakeClusterBindings) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1alpha2.ClusterBinding, err error) {
	emptyResult := &v1alpha2.ClusterBinding{}
	obj, err := c.Fake.
		Invokes(testing.NewGetActionWithOptions(clusterbindingsResource, c.ns, name, options), emptyResult)

	if obj == nil {
		return emptyResult, err
	}
	return obj.(*v1alpha2.ClusterBinding), err
}

// List takes label and field selectors, and returns the list of ClusterBindings that match those selectors.
func (c *FakeClusterBindings) List(ctx context.Context, opts v1.ListOptions) (result *v1alpha2.ClusterBindingList, err error) {
	emptyResult := &v1alpha2.ClusterBindingList{}
	obj, err := c.Fake.
		Invokes(testing.NewListActionWithOptions(clusterbindingsResource, clusterbindingsKind, c.ns, opts), emptyResult)

	if obj == nil {
		return emptyResult, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1alpha2.ClusterBindingList{ListMeta: obj.(*v1alpha2.ClusterBindingList).ListMeta}
	for _, item := range obj.(*v1alpha2.ClusterBindingList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested clusterBindings.
func (c *FakeClusterBindings) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchActionWithOptions(clusterbindingsResource, c.ns, opts))

}

// Create takes the representation of a clusterBinding and creates it.  Returns the server's representation of the clusterBinding, and an error, if there is any.
func (c *FakeClusterBindings) Create(ctx context.Context, clusterBinding *v1alpha2.ClusterBinding, opts v1.CreateOptions) (result *v1alpha2.ClusterBinding, err error) {
	emptyResult := &v1alpha2.ClusterBinding{}
	obj, err := c.Fake.
		Invokes(testing.NewCreateActionWithOptions(clusterbindingsResource, c.ns, clusterBinding, opts), emptyResult)

	if obj == nil {
		return emptyResult, err
	}
	return obj.(*v1alpha2.ClusterBinding), err
}

// Update takes the representation of a clusterBinding and updates it. Returns the server's representation of the clusterBinding, and an error, if there is any.
func (c *FakeClusterBindings) Update(ctx context.Context, clusterBinding *v1alpha2.ClusterBinding, opts v1.UpdateOptions) (result *v1alpha2.ClusterBinding, err error) {
	emptyResult := &v1alpha2.ClusterBinding{}
	obj, err := c.Fake.
		Invokes(testing.NewUpdateActionWithOptions(clusterbindingsResource, c.ns, clusterBinding, opts), emptyResult)

	if obj == nil {
		return emptyResult, err
	}
	return obj.(*v1alpha2.ClusterBinding), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakeClusterBindings) UpdateStatus(ctx context.Context, clusterBinding *v1alpha2.ClusterBinding, opts v1.UpdateOptions) (result *v1alpha2.ClusterBinding, err error) {
	emptyResult := &v1alpha2.ClusterBinding{}
	obj, err := c.Fake.
		Invokes(testing.NewUpdateSubresourceActionWithOptions(clusterbindingsResource, "status", c.ns, clusterBinding, opts), emptyResult)

	if obj == nil {
		return emptyResult, err
	}
	return obj.(*v1alpha2.ClusterBinding), err
}

// Delete takes name of the clusterBinding and deletes it. Returns an error if one occurs.
func (c *FakeClusterBindings) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteActionWithOptions(clusterbindingsResource, c.ns, name, opts), &v1alpha2.ClusterBinding{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeClusterBindings) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	action := testing.NewDeleteCollectionActionWithOptions(clusterbindingsResource, c.ns, opts, listOpts)

	_, err := c.Fake.Invokes(action, &v1alpha2.ClusterBindingList{})
	return err
}

// Patch applies the patch and returns the patched clusterBinding.
func (c *FakeClusterBindings) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha2.ClusterBinding, err error) {
	emptyResult := &v1alpha2.ClusterBinding{}
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceActionWithOptions(clusterbindingsResource, c.ns, name, pt, data, opts, subresources...), emptyResult)

	if obj == nil {
		return emptyResult, err
	}
	return obj.(*v1alpha2.ClusterBinding), err
}
