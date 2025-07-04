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

// FakeAPIServiceBindings implements APIServiceBindingInterface
type FakeAPIServiceBindings struct {
	Fake *FakeKubeBindV1alpha2
}

var apiservicebindingsResource = v1alpha2.SchemeGroupVersion.WithResource("apiservicebindings")

var apiservicebindingsKind = v1alpha2.SchemeGroupVersion.WithKind("APIServiceBinding")

// Get takes name of the aPIServiceBinding, and returns the corresponding aPIServiceBinding object, and an error if there is any.
func (c *FakeAPIServiceBindings) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1alpha2.APIServiceBinding, err error) {
	emptyResult := &v1alpha2.APIServiceBinding{}
	obj, err := c.Fake.
		Invokes(testing.NewRootGetActionWithOptions(apiservicebindingsResource, name, options), emptyResult)
	if obj == nil {
		return emptyResult, err
	}
	return obj.(*v1alpha2.APIServiceBinding), err
}

// List takes label and field selectors, and returns the list of APIServiceBindings that match those selectors.
func (c *FakeAPIServiceBindings) List(ctx context.Context, opts v1.ListOptions) (result *v1alpha2.APIServiceBindingList, err error) {
	emptyResult := &v1alpha2.APIServiceBindingList{}
	obj, err := c.Fake.
		Invokes(testing.NewRootListActionWithOptions(apiservicebindingsResource, apiservicebindingsKind, opts), emptyResult)
	if obj == nil {
		return emptyResult, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1alpha2.APIServiceBindingList{ListMeta: obj.(*v1alpha2.APIServiceBindingList).ListMeta}
	for _, item := range obj.(*v1alpha2.APIServiceBindingList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested aPIServiceBindings.
func (c *FakeAPIServiceBindings) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewRootWatchActionWithOptions(apiservicebindingsResource, opts))
}

// Create takes the representation of a aPIServiceBinding and creates it.  Returns the server's representation of the aPIServiceBinding, and an error, if there is any.
func (c *FakeAPIServiceBindings) Create(ctx context.Context, aPIServiceBinding *v1alpha2.APIServiceBinding, opts v1.CreateOptions) (result *v1alpha2.APIServiceBinding, err error) {
	emptyResult := &v1alpha2.APIServiceBinding{}
	obj, err := c.Fake.
		Invokes(testing.NewRootCreateActionWithOptions(apiservicebindingsResource, aPIServiceBinding, opts), emptyResult)
	if obj == nil {
		return emptyResult, err
	}
	return obj.(*v1alpha2.APIServiceBinding), err
}

// Update takes the representation of a aPIServiceBinding and updates it. Returns the server's representation of the aPIServiceBinding, and an error, if there is any.
func (c *FakeAPIServiceBindings) Update(ctx context.Context, aPIServiceBinding *v1alpha2.APIServiceBinding, opts v1.UpdateOptions) (result *v1alpha2.APIServiceBinding, err error) {
	emptyResult := &v1alpha2.APIServiceBinding{}
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateActionWithOptions(apiservicebindingsResource, aPIServiceBinding, opts), emptyResult)
	if obj == nil {
		return emptyResult, err
	}
	return obj.(*v1alpha2.APIServiceBinding), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakeAPIServiceBindings) UpdateStatus(ctx context.Context, aPIServiceBinding *v1alpha2.APIServiceBinding, opts v1.UpdateOptions) (result *v1alpha2.APIServiceBinding, err error) {
	emptyResult := &v1alpha2.APIServiceBinding{}
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateSubresourceActionWithOptions(apiservicebindingsResource, "status", aPIServiceBinding, opts), emptyResult)
	if obj == nil {
		return emptyResult, err
	}
	return obj.(*v1alpha2.APIServiceBinding), err
}

// Delete takes name of the aPIServiceBinding and deletes it. Returns an error if one occurs.
func (c *FakeAPIServiceBindings) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewRootDeleteActionWithOptions(apiservicebindingsResource, name, opts), &v1alpha2.APIServiceBinding{})
	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeAPIServiceBindings) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	action := testing.NewRootDeleteCollectionActionWithOptions(apiservicebindingsResource, opts, listOpts)

	_, err := c.Fake.Invokes(action, &v1alpha2.APIServiceBindingList{})
	return err
}

// Patch applies the patch and returns the patched aPIServiceBinding.
func (c *FakeAPIServiceBindings) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha2.APIServiceBinding, err error) {
	emptyResult := &v1alpha2.APIServiceBinding{}
	obj, err := c.Fake.
		Invokes(testing.NewRootPatchSubresourceActionWithOptions(apiservicebindingsResource, name, pt, data, opts, subresources...), emptyResult)
	if obj == nil {
		return emptyResult, err
	}
	return obj.(*v1alpha2.APIServiceBinding), err
}
