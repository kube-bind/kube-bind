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

	provider "github.com/kcp-dev/multicluster-provider/apiexport"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/multicluster-runtime/pkg/multicluster"
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
	// TODO: Add cluster seeding here in the follow-up PRS.
	return a.Aware.Engage(ctx, name, cluster)
}
