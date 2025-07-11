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

package v1alpha2

import (
	"context"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	gentype "k8s.io/client-go/gentype"

	v1alpha2 "github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2"
	kubebindv1alpha2 "github.com/kube-bind/kube-bind/sdk/kcp/applyconfiguration/kubebind/v1alpha2"
	scheme "github.com/kube-bind/kube-bind/sdk/kcp/clientset/versioned/scheme"
)

// APIServiceBindingsGetter has a method to return a APIServiceBindingInterface.
// A group's client should implement this interface.
type APIServiceBindingsGetter interface {
	APIServiceBindings() APIServiceBindingInterface
}

// APIServiceBindingInterface has methods to work with APIServiceBinding resources.
type APIServiceBindingInterface interface {
	Create(ctx context.Context, aPIServiceBinding *v1alpha2.APIServiceBinding, opts v1.CreateOptions) (*v1alpha2.APIServiceBinding, error)
	Update(ctx context.Context, aPIServiceBinding *v1alpha2.APIServiceBinding, opts v1.UpdateOptions) (*v1alpha2.APIServiceBinding, error)
	// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
	UpdateStatus(ctx context.Context, aPIServiceBinding *v1alpha2.APIServiceBinding, opts v1.UpdateOptions) (*v1alpha2.APIServiceBinding, error)
	Delete(ctx context.Context, name string, opts v1.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error
	Get(ctx context.Context, name string, opts v1.GetOptions) (*v1alpha2.APIServiceBinding, error)
	List(ctx context.Context, opts v1.ListOptions) (*v1alpha2.APIServiceBindingList, error)
	Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha2.APIServiceBinding, err error)
	Apply(ctx context.Context, aPIServiceBinding *kubebindv1alpha2.APIServiceBindingApplyConfiguration, opts v1.ApplyOptions) (result *v1alpha2.APIServiceBinding, err error)
	// Add a +genclient:noStatus comment above the type to avoid generating ApplyStatus().
	ApplyStatus(ctx context.Context, aPIServiceBinding *kubebindv1alpha2.APIServiceBindingApplyConfiguration, opts v1.ApplyOptions) (result *v1alpha2.APIServiceBinding, err error)
	APIServiceBindingExpansion
}

// aPIServiceBindings implements APIServiceBindingInterface
type aPIServiceBindings struct {
	*gentype.ClientWithListAndApply[*v1alpha2.APIServiceBinding, *v1alpha2.APIServiceBindingList, *kubebindv1alpha2.APIServiceBindingApplyConfiguration]
}

// newAPIServiceBindings returns a APIServiceBindings
func newAPIServiceBindings(c *KubeBindV1alpha2Client) *aPIServiceBindings {
	return &aPIServiceBindings{
		gentype.NewClientWithListAndApply[*v1alpha2.APIServiceBinding, *v1alpha2.APIServiceBindingList, *kubebindv1alpha2.APIServiceBindingApplyConfiguration](
			"apiservicebindings",
			c.RESTClient(),
			scheme.ParameterCodec,
			"",
			func() *v1alpha2.APIServiceBinding { return &v1alpha2.APIServiceBinding{} },
			func() *v1alpha2.APIServiceBindingList { return &v1alpha2.APIServiceBindingList{} }),
	}
}
