# CHANGELOG

Change log is high level summary of notable changes for each release.

## API Changes in v0.6.0 release

### Helm Chart Updates

Introduction of helm chart, `kube-bind/backend`, to deploy the backend components with configurable options.

### Catalog API
Introduction of new `Collection` and `APIServiceExportTemplate` CRDs for better service organization:
- **Collections**: Function as folders in the UI, grouping bindable resources such as templates
- **APIServiceExportTemplate**: Group multiple CRDs with their related resources and permission claims that can be used to create a `APIServiceExportRequest`

### Enhanced Permission Claims
Major improvements to `PermissionClaims` in APIServiceExportSpec:
- **NamedResources**: Specify exact resources by name and namespace
- **Combined Selectors**: Use both label selectors AND named resources (both must match)
- **Granular Control**: More precise access control for service resources

### Provider-side Namespace Management
Enhanced namespace management on the provider side:
- **APIServiceNamespace Controller**: Automatically creates Roles and RoleBindings
- **Namespace Isolation**: Each consumer gets isolated provider-side namespaces  
- **RBAC Automation**: Proper permissions created based on scope (namespaced vs cluster-scoped)
- **Namespace Pre-provisioning**: Providers can pre-create namespaces for better UX

**Important**: When `ClusterScope` mode is used, cluster-wide permissions are created instead of namespaced ones. 

## API Changes in v0.5.0 release

Version v0.5.0 includes significant architectural improvements to the API structure:

### Major Changes

- **API Version Upgrade**: Introduced `v1alpha2` API version alongside existing `v1alpha1`
- **Service Exposure Refactoring**: Refactored the service exposure mechanism from embedded CRD specifications to a resource-based model:
  - `APIServiceExportSpec` now uses `Resources []APIServiceExportResource` instead of embedded CRD specs
  - `BoundSchema`: New resource type in `v1alpha2` that represents bound schemas in consumer clusters and tracks the status of synced resources
  - This allows one APIServiceExport to reference multiple CRDs more efficiently

### Backend Architecture Improvements

- **MultiCluster Runtime Integration**: The backend now leverages `sigs.k8s.io/multicluster-runtime` for enhanced cluster management capabilities
- **Provider Support**: Built-in support for multiple backend providers including KCP through `github.com/kcp-dev/multicluster-provider`
- **Enhanced Cluster Operations**: Improved cluster-aware resource management with dedicated manager architecture for handling multi-cluster scenarios
