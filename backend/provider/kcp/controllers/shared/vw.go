/*
Copyright 2026 The Kube Bind Authors.

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

// Package shared provides utilities shared across kcp controllers.
package shared

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"sync"

	apisv1alpha1 "github.com/kcp-dev/sdk/apis/apis/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// AnnotationOwnerBinding is the annotation key used to link an
// APIServiceExportTemplate back to the APIBinding that owns it.
const AnnotationOwnerBinding = "apibindingtemplate.kube-bind.io/owner-binding"

// ExtractClusterID extracts the cluster ID from an apiexport virtual workspace
// URL. The expected URL format (as of kcp v0.26+) is:
//
//	https://host:port/services/apiexport/<path>/<apiexport-name>/clusters/<cluster-id>
//
// where path segments 0-3 are /services/apiexport/<workspace-path>/<export>,
// segment 4 is "clusters", and segment 5 is the cluster ID.
func ExtractClusterID(clusterConfig *rest.Config) (string, error) {
	u, err := url.Parse(clusterConfig.Host)
	if err != nil {
		return "", fmt.Errorf("failed to parse cluster host URL: %w", err)
	}

	pathParts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(pathParts) < 6 || pathParts[4] != "clusters" {
		return "", fmt.Errorf("unexpected apiexport URL format: %s", u.Path)
	}

	return pathParts[5], nil
}

// VWClientCache caches controller-runtime clients keyed by cluster ID to avoid
// creating a new client on every schema lookup.
type VWClientCache struct {
	mu         sync.RWMutex
	clients    map[string]client.Client
	baseConfig *rest.Config
	scheme     *runtime.Scheme
}

// NewVWClientCache creates a new VWClientCache.
func NewVWClientCache(baseConfig *rest.Config, scheme *runtime.Scheme) *VWClientCache {
	return &VWClientCache{
		clients:    make(map[string]client.Client),
		baseConfig: baseConfig,
		scheme:     scheme,
	}
}

// GetClient returns a cached client for the given cluster ID, creating one if
// necessary.
func (c *VWClientCache) GetClient(clusterID string) (client.Client, error) {
	c.mu.RLock()
	if cl, ok := c.clients[clusterID]; ok {
		c.mu.RUnlock()
		return cl, nil
	}
	c.mu.RUnlock()

	c.mu.Lock()
	defer c.mu.Unlock()

	// Double-check after acquiring write lock.
	if cl, ok := c.clients[clusterID]; ok {
		return cl, nil
	}

	cl, err := newVWClient(c.baseConfig, c.scheme, clusterID)
	if err != nil {
		return nil, err
	}
	c.clients[clusterID] = cl
	return cl, nil
}

// newVWClient creates a client pointing at the apiresourceschema virtual workspace
// for the given cluster ID:
// https://host:port/services/apiresourceschema/{clusterID}/clusters/*/
func newVWClient(baseConfig *rest.Config, scheme *runtime.Scheme, clusterID string) (client.Client, error) {
	cfg := rest.CopyConfig(baseConfig)
	u, err := url.Parse(cfg.Host)
	if err != nil {
		return nil, fmt.Errorf("failed to parse base config host: %w", err)
	}

	u.Path = fmt.Sprintf("/services/apiresourceschema/%s/clusters/*", clusterID)
	cfg.Host = u.String()

	return client.New(cfg, client.Options{Scheme: scheme})
}

// SchemaGetterWithFallback returns a function that first tries to get an
// APIResourceSchema from the workspace client, and if not found, falls back to
// the apiresourceschema virtual workspace via the cached VW client.
func SchemaGetterWithFallback(
	workspaceClient client.Client,
	clusterConfig *rest.Config,
	vwCache *VWClientCache,
) func(ctx context.Context, name string) (*apisv1alpha1.APIResourceSchema, error) {
	return func(ctx context.Context, name string) (*apisv1alpha1.APIResourceSchema, error) {
		logger := log.FromContext(ctx)

		// 1. Try the current workspace first.
		var schema apisv1alpha1.APIResourceSchema
		err := workspaceClient.Get(ctx, client.ObjectKey{Name: name}, &schema)
		if err == nil {
			return &schema, nil
		}
		if !errors.IsNotFound(err) {
			return nil, err
		}

		// 2. Fallback: try the apiresourceschema virtual workspace.
		logger.V(2).Info("APIResourceSchema not found in workspace, trying VW fallback", "schema", name)

		clusterID, err := ExtractClusterID(clusterConfig)
		if err != nil {
			return nil, fmt.Errorf("cannot build VW fallback client: %w", err)
		}

		vwClient, err := vwCache.GetClient(clusterID)
		if err != nil {
			return nil, fmt.Errorf("failed to get VW client for cluster %q: %w", clusterID, err)
		}

		var vwSchema apisv1alpha1.APIResourceSchema
		if err := vwClient.Get(ctx, client.ObjectKey{Name: name}, &vwSchema); err != nil {
			return nil, fmt.Errorf("APIResourceSchema %q not found in workspace or VW: %w", name, err)
		}

		return &vwSchema, nil
	}
}
