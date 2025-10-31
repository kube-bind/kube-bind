# Backend

The kube-bind backend provides service export and binding capabilities for single Kubernetes clusters acting as backend or many clusters with support for multiple cluster providers through the multicluster-runtime architecture.

## Architecture

Starting with v0.5.0, the backend leverages `sigs.k8s.io/multicluster-runtime` for enhanced cluster management capabilities.

### Key Components

- **MultiCluster Runtime Integration**: Built on `sigs.k8s.io/multicluster-runtime` for provider-agnostic cluster operations
- **Provider Support**: Extensible provider system supporting different backend implementations
- **Manager Architecture**: Uses `mcmanager.Manager` for cluster-aware resource management

### Supported Providers

- **Default Provider**: Standard Kubernetes cluster support
- **kcp Provider**: Integration with [kcp](https://www.kcp.io/) through `github.com/kcp-dev/multicluster-provider`

## Configuration

The backend can be configured to use different providers:

```bash
./bin/backend \
  --multicluster-runtime-provider kcp \
  --server-url=$(kubectl get apiexportendpointslice kube-bind.io -o jsonpath="{.status.endpoints[0].url}") \
  # ... other options
```

### Provider Configuration

#### kcp Provider

When using the kcp provider (`--multicluster-runtime-provider kcp`), the backend:

- Connects to kcp workspaces through APIExports
- Manages resources across logical clusters
- Supports advanced multi-tenancy features
- Enables workspace-based isolation

#### Default Provider

The default provider works with standard Kubernetes clusters and provides:

- Direct cluster connectivity
- Namespace-based isolation
- Standard RBAC integration

## API Changes

The backend now supports the v1alpha2 API with significant architectural improvements:

- **Resource-Based Exports**: APIServiceExport now uses resource references instead of embedded CRDs
- **BoundSchema Support**: Integration with BoundSchema resources for better schema management
- **Multi-Resource Support**: Single exports can reference multiple CRDs efficiently

## Controllers

The backend includes several controllers for managing the export/binding lifecycle:

- **ClusterBinding Controller**: Manages cluster binding lifecycle
- **ServiceExport Controller**: Handles APIServiceExport resources
- **ServiceExportRequest Controller**: Processes export requests
- **ServiceNamespace Controller**: Manages namespace isolation
