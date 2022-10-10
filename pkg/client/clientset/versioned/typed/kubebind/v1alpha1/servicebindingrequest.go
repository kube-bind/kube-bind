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

package v1alpha1

import (
	"context"
	"time"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"

	v1alpha1 "github.com/kube-bind/kube-bind/pkg/apis/kubebind/v1alpha1"
	scheme "github.com/kube-bind/kube-bind/pkg/client/clientset/versioned/scheme"
)

// ServiceBindingRequestsGetter has a method to return a ServiceBindingRequestInterface.
// A group's client should implement this interface.
type ServiceBindingRequestsGetter interface {
	ServiceBindingRequests(namespace string) ServiceBindingRequestInterface
}

// ServiceBindingRequestInterface has methods to work with ServiceBindingRequest resources.
type ServiceBindingRequestInterface interface {
	Create(ctx context.Context, serviceBindingRequest *v1alpha1.ServiceBindingRequest, opts v1.CreateOptions) (*v1alpha1.ServiceBindingRequest, error)
	Update(ctx context.Context, serviceBindingRequest *v1alpha1.ServiceBindingRequest, opts v1.UpdateOptions) (*v1alpha1.ServiceBindingRequest, error)
	UpdateStatus(ctx context.Context, serviceBindingRequest *v1alpha1.ServiceBindingRequest, opts v1.UpdateOptions) (*v1alpha1.ServiceBindingRequest, error)
	Delete(ctx context.Context, name string, opts v1.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error
	Get(ctx context.Context, name string, opts v1.GetOptions) (*v1alpha1.ServiceBindingRequest, error)
	List(ctx context.Context, opts v1.ListOptions) (*v1alpha1.ServiceBindingRequestList, error)
	Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.ServiceBindingRequest, err error)
	ServiceBindingRequestExpansion
}

// serviceBindingRequests implements ServiceBindingRequestInterface
type serviceBindingRequests struct {
	client rest.Interface
	ns     string
}

// newServiceBindingRequests returns a ServiceBindingRequests
func newServiceBindingRequests(c *KubeBindV1alpha1Client, namespace string) *serviceBindingRequests {
	return &serviceBindingRequests{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Get takes name of the serviceBindingRequest, and returns the corresponding serviceBindingRequest object, and an error if there is any.
func (c *serviceBindingRequests) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1alpha1.ServiceBindingRequest, err error) {
	result = &v1alpha1.ServiceBindingRequest{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("servicebindingrequests").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do(ctx).
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of ServiceBindingRequests that match those selectors.
func (c *serviceBindingRequests) List(ctx context.Context, opts v1.ListOptions) (result *v1alpha1.ServiceBindingRequestList, err error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	result = &v1alpha1.ServiceBindingRequestList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("servicebindingrequests").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Do(ctx).
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested serviceBindingRequests.
func (c *serviceBindingRequests) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("servicebindingrequests").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Watch(ctx)
}

// Create takes the representation of a serviceBindingRequest and creates it.  Returns the server's representation of the serviceBindingRequest, and an error, if there is any.
func (c *serviceBindingRequests) Create(ctx context.Context, serviceBindingRequest *v1alpha1.ServiceBindingRequest, opts v1.CreateOptions) (result *v1alpha1.ServiceBindingRequest, err error) {
	result = &v1alpha1.ServiceBindingRequest{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("servicebindingrequests").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(serviceBindingRequest).
		Do(ctx).
		Into(result)
	return
}

// Update takes the representation of a serviceBindingRequest and updates it. Returns the server's representation of the serviceBindingRequest, and an error, if there is any.
func (c *serviceBindingRequests) Update(ctx context.Context, serviceBindingRequest *v1alpha1.ServiceBindingRequest, opts v1.UpdateOptions) (result *v1alpha1.ServiceBindingRequest, err error) {
	result = &v1alpha1.ServiceBindingRequest{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("servicebindingrequests").
		Name(serviceBindingRequest.Name).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(serviceBindingRequest).
		Do(ctx).
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *serviceBindingRequests) UpdateStatus(ctx context.Context, serviceBindingRequest *v1alpha1.ServiceBindingRequest, opts v1.UpdateOptions) (result *v1alpha1.ServiceBindingRequest, err error) {
	result = &v1alpha1.ServiceBindingRequest{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("servicebindingrequests").
		Name(serviceBindingRequest.Name).
		SubResource("status").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(serviceBindingRequest).
		Do(ctx).
		Into(result)
	return
}

// Delete takes name of the serviceBindingRequest and deletes it. Returns an error if one occurs.
func (c *serviceBindingRequests) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("servicebindingrequests").
		Name(name).
		Body(&opts).
		Do(ctx).
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *serviceBindingRequests) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	var timeout time.Duration
	if listOpts.TimeoutSeconds != nil {
		timeout = time.Duration(*listOpts.TimeoutSeconds) * time.Second
	}
	return c.client.Delete().
		Namespace(c.ns).
		Resource("servicebindingrequests").
		VersionedParams(&listOpts, scheme.ParameterCodec).
		Timeout(timeout).
		Body(&opts).
		Do(ctx).
		Error()
}

// Patch applies the patch and returns the patched serviceBindingRequest.
func (c *serviceBindingRequests) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.ServiceBindingRequest, err error) {
	result = &v1alpha1.ServiceBindingRequest{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("servicebindingrequests").
		Name(name).
		SubResource(subresources...).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(data).
		Do(ctx).
		Into(result)
	return
}
