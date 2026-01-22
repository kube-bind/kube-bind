# Backend-Only Bindings: Bundle-Based Service Discovery

**Status**: Implemented  
**Author**: Based on implementation in PR  
**Date**: January 2025  

## Overview

This proposal introduces a "backend-only" binding mode for kube-bind that eliminates the need for interactive OIDC-based authentication flows and enables automatic service discovery. The core feature is the `APIServiceBindingBundle` resource, which allows consumer clusters to automatically discover and bind to all available API services from a provider cluster using only a kubeconfig.

## Motivation

### Current Handshake Process

The traditional kube-bind binding flow requires:

1. **Manual OIDC Authentication**: User authenticates via provider's web UI
2. **Service Selection**: User browses and selects specific services to bind
3. **Per-Service Binding**: Each service requires individual `APIServiceBinding` creation
4. **Request-Response Cycle**: Consumer creates `BindableResourcesRequest` → Provider processes → Response returned

This creates friction for:
- **Automated/Scripted Deployments**: CI/CD pipelines and infrastructure-as-code cannot easily orchestrate bindings
- **Service Discovery**: Consumers must know what services exist before binding
- **Bulk Operations**: Binding multiple services requires repeated handshake cycles
- **Headless Environments**: Servers without browsers cannot complete OIDC flows

### Desired Backend-Only Mode

Enable scenarios where:
- Consumer has direct cluster-to-cluster kubeconfig access
- No human interaction needed for service binding
- All available services are discovered and bound automatically
- Changes in provider services are synchronized without manual intervention

## Proposal

### New Resource: APIServiceBindingBundle

A cluster-scoped resource that represents a bundle of all API services from a provider cluster:

```yaml
apiVersion: kube-bind.io/v1alpha2
kind: APIServiceBindingBundle
metadata:
  name: prod-services
spec:
  kubeconfigSecretRef:
    name: provider-kubeconfig
    namespace: kube-bind
    key: kubeconfig
status:
  conditions:
    - type: SecretValid
      status: "True"
    - type: Synced
      status: "True"
```

### Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│ Consumer Cluster                                                │
│                                                                 │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │ APIServiceBindingBundle                                 │   │
│  │   spec.kubeconfigSecretRef: provider-kubeconfig         │   │
│  └───────────────────┬─────────────────────────────────────┘   │
│                      │                                          │
│                      ▼                                          │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │ ServiceBindingBundle Controller                         │   │
│  │   • Validates kubeconfig secret                         │   │
│  │   • Polls provider every 15s                            │   │
│  │   • Lists APIServiceExports                             │   │
│  │   • Creates/Updates/Deletes APIServiceBindings          │   │
│  └───────────────────┬─────────────────────────────────────┘   │
│                      │                                          │
│                      ▼                                          │
│  ┌────────────────────────────────────────┐                    │
│  │ APIServiceBinding (auto-created)       │                    │
│  │   ownerReferences:                     │                    │
│  │     - APIServiceBindingBundle          │                    │
│  │   spec:                                │                    │
│  │     kubeconfigSecretRef: ...           │                    │
│  └────────────────────────────────────────┘                    │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
                       │
                       │ kubeconfig
                       │ (polling connection)
                       ▼
┌─────────────────────────────────────────────────────────────────┐
│ Provider Cluster                                                │
│                                                                 │
│  ┌────────────────────────────────────────┐                    │
│  │ APIServiceExport (namespace-scoped)    │                    │
│  │   • Discovered by polling              │                    │
│  │   • No authentication required         │                    │
│  └────────────────────────────────────────┘                    │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### Controller Behavior

**ServiceBindingBundle Controller** (new):

1. **Secret Validation Phase**
   - Watches `APIServiceBindingBundle` resources
   - Validates referenced kubeconfig secret exists and is valid
   - Extracts provider cluster REST config and namespace
   - Sets `SecretValid` condition

2. **Service Discovery Phase**
   - Creates provider cluster client from kubeconfig
   - Lists `APIServiceExport` resources in provider namespace
   - Polls every 15 seconds (configurable via `--provider-polling-interval`)

3. **Binding Synchronization Phase**
   - For each discovered export, creates an `APIServiceBinding`
   - Sets `OwnerReference` pointing to bundle (for garbage collection)
   - Uses same kubeconfig reference as bundle spec
   - Deletes bindings for exports that no longer exist
   - Sets `Synced` condition

**Existing ServiceBinding Controller**:
- Unchanged - continues to reconcile individual `APIServiceBinding` resources
- Works for both manually-created bindings and bundle-generated bindings

### Key Implementation Details

#### 1. OwnerReferences for Garbage Collection

```go
binding := &kubebindv1alpha2.APIServiceBinding{
    ObjectMeta: metav1.ObjectMeta{
        Name: export.Name,
        OwnerReferences: []metav1.OwnerReference{
            {
                APIVersion: kubebindv1alpha2.SchemeGroupVersion.String(),
                Kind:       kubebindv1alpha2.KindAPIServiceBindingBundle,
                Name:       bundle.Name,
                UID:        bundle.UID,
                Controller: func() *bool { b := true; return &b }(),
            },
        },
    },
    Spec: kubebindv1alpha2.APIServiceBindingSpec{
        KubeconfigSecretRef: bundle.Spec.KubeconfigSecretRef,
    },
}
```

Benefits:
- Deleting bundle automatically cleans up all child bindings
- No orphaned resources left behind
- Follows Kubernetes garbage collection patterns

#### 2. Polling Instead of Watching

```go
ticker := time.NewTicker(15 * time.Second)
for {
    select {
    case <-ticker.C:
        // Re-queue all bundles for reconciliation
        bundles, _ := lister.List(labels.Everything())
        for _, bundle := range bundles {
            queue.Add(bundle)
        }
    }
}
```

Rationale:
- Simpler than cross-cluster watch
- Provider cluster doesn't need to push notifications
- Configurable interval balances freshness vs load
- Future: could upgrade to watch-based for real-time updates

#### 3. Namespace Scoping from Kubeconfig

The controller extracts the namespace from the kubeconfig's current context:

```go
cfg, _ := clientcmd.Load(kubeconfigBytes)
kubeContext := cfg.Contexts[cfg.CurrentContext]
namespace := kubeContext.Namespace  // Use this for ListAPIServiceExports
```

This ensures:
- Different namespaces for different services
- Multi-tenancy support
- Explicit isolation via kubeconfig scoping

#### 4. CRD Relaxation

Previously, `kubeconfigSecretRef.key` was hardcoded to `"kubeconfig"`. Now it's flexible:

```go
// Before:
// +kubebuilder:validation:Enum=kubeconfig
Key string `json:"key"`

// After:
Key string `json:"key"`
```

This allows custom key names in secrets, improving compatibility.

### New Files

| File | Purpose | Lines |
|------|---------|-------|
| [pkg/konnector/controllers/servicebindingbundle/servicebindingbundle_controller.go](pkg/konnector/controllers/servicebindingbundle/servicebindingbundle_controller.go) | Main controller structure, event handlers, polling logic | 313 |
| [pkg/konnector/controllers/servicebindingbundle/servicebindingbundle_reconcile.go](pkg/konnector/controllers/servicebindingbundle/servicebindingbundle_reconcile.go) | Reconciliation logic, secret validation, binding sync | 307 |
| [pkg/indexers/servicebindingbundle.go](pkg/indexers/servicebindingbundle.go) | Indexer for efficient secret-to-bundle lookups | 39 |
| [sdk/apis/kubebind/v1alpha2/apiservicebindingbundle_types.go](sdk/apis/kubebind/v1alpha2/apiservicebindingbundle_types.go) | Type definition for APIServiceBindingBundle CRD | 83 |
| [deploy/crd/kube-bind.io_apiservicebindingbundles.yaml](deploy/crd/kube-bind.io_apiservicebindingbundles.yaml) | CRD manifest for installation | 149 |

### Modified Files

| File | Change Summary |
|------|----------------|
| [pkg/konnector/konnector_controller.go](pkg/konnector/konnector_controller.go) | Instantiate and start ServiceBindingBundle controller |
| [pkg/konnector/server.go](pkg/konnector/server.go) | Install APIServiceBindingBundle CRD on startup |
| [pkg/konnector/config.go](pkg/konnector/config.go) | Add `ProviderPollingInterval` configuration |
| [pkg/konnector/options/options.go](pkg/konnector/options/options.go) | Add `--provider-polling-interval` flag (default: 15s) |
| [sdk/apis/kubebind/v1alpha2/apiservicebinding_types.go](sdk/apis/kubebind/v1alpha2/apiservicebinding_types.go) | Remove enum constraint on `kubeconfigSecretRef.key` |

Total: **~950 lines** of new code + ~15 files of generated client code

## Benefits

### 1. Eliminates Interactive Handshake

**Before**: 
- User visits `https://provider.example.com/export`
- Clicks "Login with OIDC"
- Selects services from UI
- Copies kubectl commands
- Runs commands on consumer cluster

**After**:
```bash
# One-time setup
kubectl create secret generic provider-kubeconfig \
  --from-file=kubeconfig=/path/to/provider.kubeconfig \
  -n kube-bind

# Automatic binding of ALL services
kubectl apply -f - <<EOF
apiVersion: kube-bind.io/v1alpha2
kind: APIServiceBindingBundle
metadata:
  name: prod-services
spec:
  kubeconfigSecretRef:
    name: provider-kubeconfig
    namespace: kube-bind
    key: kubeconfig
EOF
```

### 2. Continuous Service Discovery

- New services added to provider → automatically bound to consumer
- Services removed from provider → automatically unbound from consumer
- No manual intervention required
- Polling interval keeps bindings fresh (15s default)

### 3. GitOps-Friendly

```yaml
# Single declarative resource in Git
apiVersion: kube-bind.io/v1alpha2
kind: APIServiceBindingBundle
metadata:
  name: prod-services
spec:
  kubeconfigSecretRef:
    name: provider-kubeconfig
    namespace: kube-bind
    key: kubeconfig
```

Argo CD / Flux can manage bundles alongside other infrastructure resources.

### 4. Multi-Provider Support

```yaml
# Bind to multiple provider clusters
---
apiVersion: kube-bind.io/v1alpha2
kind: APIServiceBindingBundle
metadata:
  name: prod-provider
spec:
  kubeconfigSecretRef:
    name: prod-kubeconfig
    namespace: kube-bind
    key: kubeconfig
---
apiVersion: kube-bind.io/v1alpha2
kind: APIServiceBindingBundle
metadata:
  name: staging-provider
spec:
  kubeconfigSecretRef:
    name: staging-kubeconfig
    namespace: kube-bind
    key: kubeconfig
```

### 5. Reduced Cognitive Load

- No need to remember service names
- No per-service configuration
- Single point of management

## Comparison: Traditional vs Backend-Only

| Aspect | Traditional (OIDC + Web UI) | Backend-Only (Bundle) |
|--------|------------------------------|------------------------|
| **Authentication** | OIDC via browser | Kubeconfig-based |
| **Service Discovery** | Manual selection in UI | Automatic via polling |
| **Binding Scope** | Per-service | All services (wildcard) |
| **Human Interaction** | Required | Optional |
| **Automation** | Difficult | Native |
| **GitOps** | Awkward | Natural |
| **Multi-Service** | Repeated handshakes | Single bundle |
| **Service Updates** | Manual re-binding | Automatic sync |
| **Use Cases** | Developer workstations, demos | Production, CI/CD, IaC |

## Future Enhancements

### 1. Watch-Based Updates (instead of polling)

```go
// Instead of ticker-based polling
providerClient.Watch(ctx, metav1.ListOptions{
    ResourceVersion: lastSeen,
})
```

Benefits: Real-time updates, reduced latency, lower load

### 2. Selective Binding (Label Selectors)

```yaml
spec:
  kubeconfigSecretRef: ...
  selector:
    matchLabels:
      environment: production
      tier: frontend
```

### 3. Health Monitoring

```yaml
status:
  conditions:
    - type: ProviderReachable
      status: "True"
      lastProbeTime: "2025-01-26T10:30:00Z"
  lastSyncTime: "2025-01-26T10:30:15Z"
  discoveredServices: 12
  boundServices: 12
```

### 4. Rate Limiting & Backoff

```go
spec:
  pollingInterval: 30s
  maxRetries: 5
  backoffMultiplier: 2.0
```

### 5. BindableResourcesRequest as CRD

Support creating `BindableResourcesRequest` directly as a CRD (instead of only via HTTP API):

```yaml
apiVersion: kube-bind.io/v1alpha2
kind: BindableResourcesRequest
metadata:
  name: request-prod-services
  namespace: kube-bind-provider-ns
spec:
  templateRef:
    name: prod-template
  clusterIdentity:
    identity: "consumer-cluster-123"
  author: "automation@example.com"
  kubeconfigSecretRef:
    name: response-secret
    key: kubeconfig
status:
  phase: Succeeded
  kubeconfigSecretRef:
    name: response-secret
    key: kubeconfig
```

This was partially implemented alongside bundles to support headless binding flows.

## Rollout Plan

### Phase 1: Foundation (Completed ✓)
- [x] Implement `APIServiceBindingBundle` CRD
- [x] Implement ServiceBindingBundle controller
- [x] Add polling mechanism
- [x] Integration with existing ServiceBinding controller
- [x] CRD installation in backend startup

### Phase 2: Testing & Documentation
- [ ] E2E tests for bundle creation and synchronization
- [ ] Unit tests for controller reconciliation logic
- [ ] User guide: "Getting Started with Backend-Only Bindings"
- [ ] Migration guide: "Converting from traditional to bundle-based"

### Phase 3: Enhancements
- [ ] Watch-based provider synchronization
- [ ] Label selectors for service filtering
- [ ] Metrics and monitoring dashboards
- [ ] Helm chart values for polling configuration

## Security Considerations

### Kubeconfig Secret Protection

The kubeconfig contains cluster credentials:
- **Must** be stored in a Secret
- **Should** have restrictive RBAC (only konnector can read)
- **Consider** using short-lived tokens via OIDC
- **Rotate** credentials regularly

### Network Segmentation

Provider cluster is accessed directly:
- Ensure network policies allow konnector → provider traffic
- Consider mutual TLS for cluster-to-cluster communication
- Use service meshes for encryption in transit

### RBAC on Provider

The kubeconfig should have minimal permissions:
```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: kube-bind-reader
  namespace: kube-bind-exports
rules:
- apiGroups: ["kube-bind.io"]
  resources: ["apiserviceexports"]
  verbs: ["get", "list", "watch"]
```

## Backward Compatibility

- Traditional OIDC-based bindings continue to work unchanged
- No breaking changes to existing APIs
- `APIServiceBinding` can be created manually or via bundle
- Controllers are orthogonal: one manages bundles, one manages bindings

## Alternatives Considered

### 1. Extend APIServiceBinding with Wildcard

```yaml
spec:
  serviceSelector: "*"  # Bind all services
```

**Rejected**: APIServiceBinding is 1:1 with exports; changing semantics would be confusing.

### 2. Controller in Provider Cluster

Have provider push bindings to consumer.

**Rejected**: Violates trust boundary; consumer should pull, not provider push.

### 3. Single Global Bundle

Only allow one bundle per cluster.

**Rejected**: Multi-provider scenarios require multiple bundles.

## Conclusion

The `APIServiceBindingBundle` feature enables **backend-only binding mode** by:

1. **Eliminating manual handshake**: No OIDC, no web UI, just kubeconfig
2. **Automating service discovery**: All services bound automatically
3. **Enabling GitOps**: Declarative resource for version control
4. **Reducing friction**: Single resource replaces N binding requests

This unlocks kube-bind for production automation, CI/CD pipelines, and infrastructure-as-code scenarios while maintaining full backward compatibility with traditional flows.

**Implementation**: ~950 lines of production code, fully integrated with existing controllers, ready for testing and documentation.
