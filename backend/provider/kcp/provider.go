/*
Copyright 2025 The Kube Bind Authors.

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

package kcp

import (
	"context"
	"fmt"
	"math"
	"net/url"
	"strings"
	"time"

	provider "github.com/kcp-dev/multicluster-provider/apiexport"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/multicluster-runtime/pkg/multicluster"

	kuberesources "github.com/kube-bind/kube-bind/backend/kubernetes/resources"
	kubebindv1alpha2 "github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2"
)

var _ multicluster.Provider = &Provider{}
var _ multicluster.ProviderRunnable = &Provider{}

// Provider is a [sigs.k8s.io/multicluster-runtime/pkg/multicluster.Provider] that represents each [logical cluster]
// (in the kcp sense) exposed via a APIExport virtual workspace as a cluster in the [sigs.k8s.io/multicluster-runtime] sense.
//
// Core functionality is delegated to the apiexport.Provider, this wrapper just adds best effort path-awareness index to it via hooks.
//
// [logical cluster]: https://docs.kcp.io/kcp/latest/concepts/terminology/#logical-cluster
type Provider struct {
	*provider.Provider
}

// New is simple provider wrapper for kcp.
func New(cfg *rest.Config, endpointSliceName string, options provider.Options) (*Provider, error) {
	p, err := provider.New(cfg, endpointSliceName, options)
	if err != nil {
		return nil, err
	}

	return &Provider{
		Provider: p,
	}, nil
}

// Get returns the cluster with the given name as a cluster.Cluster.
func (p *Provider) Get(ctx context.Context, clusterName string) (cluster.Cluster, error) {
	return p.Provider.Get(ctx, clusterName)
}

// IndexField adds an indexer to the clusters managed by this provider.
func (p *Provider) IndexField(ctx context.Context, obj client.Object, field string, extractValue client.IndexerFunc) error {
	return p.Provider.IndexField(ctx, obj, field, extractValue)
}

// Start starts the provider and blocks.
func (p *Provider) Start(ctx context.Context, aware multicluster.Aware) error {
	return p.Provider.Start(ctx, &awareWrapper{Aware: aware})
}

// awareWrapper wraps a multicluster.Aware to create kube-bind Cluster objects
// so we could bootstrap RBAC for them.
type awareWrapper struct {
	multicluster.Aware
}

func (a *awareWrapper) Engage(ctx context.Context, name string, cluster cluster.Cluster) error {
	ctx, cancel := context.WithCancel(ctx) //nolint:govet // cancel is called in the error case only.

	err := a.Aware.Engage(ctx, name, cluster)
	if err != nil {
		cancel()
		return err
	}

	cl := cluster.GetClient()

	obj := &kubebindv1alpha2.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: kubebindv1alpha2.DefaultClusterName,
		},
		Spec: kubebindv1alpha2.ClusterSpec{},
	}

	const maxRetries = 3
	for attempt := 0; attempt < maxRetries; attempt++ {
		err = cl.Create(ctx, obj)
		if err == nil || apierrors.IsAlreadyExists(err) {
			break
		}

		if !apierrors.IsForbidden(err) {
			cancel()
			return err
		}

		if attempt < maxRetries-1 {
			backoff := time.Duration(math.Pow(2, float64(attempt))) * time.Second
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				cancel()
				return ctx.Err()
			}
		}
	}

	if err != nil && !apierrors.IsAlreadyExists(err) {
		cancel()
		return err
	}
	return nil //nolint:govet // cancel is called in the error case only.
}

// NewKCPExternalAddressGenerator returns an ExternalAddressGeneratorFunc
// suitable for kcp-based clusters.
func NewKCPExternalAddressGenerator(externalAddress string) (kuberesources.ExternalAddressGeneratorFunc, error) {
	var extURL *url.URL
	if externalAddress != "" {
		var err error

		extURL, err = url.Parse(externalAddress)
		if err != nil {
			return nil, fmt.Errorf("invalid --external-address: %w", err)
		}
	}

	return func(_ context.Context, clusterConfig *rest.Config) (string, error) {
		// In kcp case, we are talking via apiexport so clientconfig will be pointing to
		// https://192.168.2.166:6443/services/apiexport/root:org:ws/<apiexport-name>/clusters/2p0rtkf7b697s6mj
		// We need to extract host and /clusters/... part
		u, err := url.Parse(clusterConfig.Host)
		if err != nil {
			return "", err
		}

		// Extract cluster ID from the path
		// Path format: /services/apiexport/root:org:ws/<apiexport-name>/clusters/{cluster-id}
		pathParts := strings.Split(strings.Trim(u.Path, "/"), "/")
		if len(pathParts) < 6 || pathParts[4] != "clusters" {
			return "", fmt.Errorf("invalid apiexport URL format")
		}

		clusterID := pathParts[5]

		// Construct new URL with cluster path
		var finalURL = u
		if extURL != nil {
			finalURL = extURL
		}

		finalURL.Path = "/clusters/" + clusterID

		return finalURL.String(), nil
	}, nil
}
