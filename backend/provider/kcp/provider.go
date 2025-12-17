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
	"math"
	"time"

	"github.com/kcp-dev/logicalcluster/v3"
	provider "github.com/kcp-dev/multicluster-provider/apiexport"
	"github.com/kcp-dev/multicluster-provider/pkg/handlers"
	"github.com/kcp-dev/multicluster-provider/pkg/paths"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/multicluster-runtime/pkg/multicluster"

	kubebindv1alpha2 "github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2"
)

var _ multicluster.Provider = &Provider{}
var _ multicluster.ProviderRunnable = &Provider{}
var _ handlers.Handler = &pathHandler{}

const (
	// LogicalClusterPathAnnotationKey is the annotation key for the logical cluster path
	// put on objects that are referenced by path by other objects.
	//
	// If this annotation exists, the system will maintain the annotation value.
	LogicalClusterPathAnnotationKey = "kcp.io/path"
)

// Provider is a [sigs.k8s.io/multicluster-runtime/pkg/multicluster.Provider] that represents each [logical cluster]
// (in the kcp sense) exposed via a APIExport virtual workspace as a cluster in the [sigs.k8s.io/multicluster-runtime] sense.
//
// Core functionality is delegated to the apiexport.Provider, this wrapper just adds best effort path-awareness index to it via hooks.
//
// [logical cluster]: https://docs.kcp.io/kcp/latest/concepts/terminology/#logical-cluster
type Provider struct {
	*provider.Provider
	// pathStore maps logical cluster paths to cluster names.
	pathStore *paths.Store
}

// New creates a new kcp virtual workspace provider. The provided [rest.Config]
// must point to a virtual workspace apiserver base path, i.e. up to but without
// the '/clusters/*' suffix. This information can be extracted from an APIExportEndpointSlice status.
func New(cfg *rest.Config, endpointSliceName string, options provider.Options) (*Provider, error) {
	store := paths.New()

	h := &pathHandler{
		pathStore: store,
	}
	options.Handlers = append(options.Handlers, h)

	p, err := provider.New(cfg, endpointSliceName, options)
	if err != nil {
		return nil, err
	}

	return &Provider{
		Provider:  p,
		pathStore: store,
	}, nil
}

// Get returns the cluster with the given name as a cluster.Cluster.
func (p *Provider) Get(ctx context.Context, clusterName string) (cluster.Cluster, error) {
	if p.pathStore != nil {
		if lcName, exists := p.pathStore.Get(clusterName); exists {
			clusterName = lcName.String()
		}
	}
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
	err := a.Aware.Engage(ctx, name, cluster)
	if err != nil {
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
			return err
		}

		if attempt < maxRetries-1 {
			backoff := time.Duration(math.Pow(2, float64(attempt))) * time.Second
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}

	if err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}
	return nil
}

type pathHandler struct {
	pathStore *paths.Store
}

func (p *pathHandler) OnAdd(obj client.Object) {
	cluster := logicalcluster.From(obj)

	path := obj.GetAnnotations()[LogicalClusterPathAnnotationKey]
	if path == "" {
		return
	}

	p.pathStore.Add(path, cluster)
}

func (p *pathHandler) OnUpdate(oldObj, newObj client.Object) {
	// Not used.
}

func (p *pathHandler) OnDelete(obj client.Object) {
	path, ok := obj.GetAnnotations()[LogicalClusterPathAnnotationKey]
	if !ok {
		return
	}

	p.pathStore.Remove(path)
}
