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

package plugin

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	"github.com/kube-bind/kube-bind/deploy/konnector"
	bindclient "github.com/kube-bind/kube-bind/pkg/client/clientset/versioned"
)

// nolint: unused
func (b *BindAPIServiceOptions) deployKonnector(ctx context.Context, config *rest.Config) error {
	logger := klog.FromContext(ctx)

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return err
	}
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return err
	}
	bindClient, err := bindclient.NewForConfig(config)
	if err != nil {
		return err
	}

	if !b.SkipKonnector {
		logger.V(1).Info("Deploying konnector")
		if err := konnector.Bootstrap(ctx, discoveryClient, dynamicClient, sets.NewString()); err != nil {
			return err
		}
	}
	first := true
	return wait.PollImmediateInfiniteWithContext(ctx, 1*time.Second, func(ctx context.Context) (bool, error) {
		_, err := bindClient.KubeBindV1alpha1().APIServiceBindings().List(ctx, metav1.ListOptions{})
		if err != nil {
			logger.V(2).Info("Waiting for APIServiceBindings to be served", "error", err, "host", bindClient.RESTClient())
			if first {
				fmt.Fprint(b.Options.IOStreams.ErrOut, "Waiting for the konnector to be ready") // nolint: errcheck
				first = false
			} else {
				fmt.Fprint(b.Options.IOStreams.ErrOut, ".") // nolint: errcheck
			}
		}
		return err == nil, nil
	})
}
