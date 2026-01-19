# Architecture Overview

This document provides detailed architecture diagrams and explanations for how kube-bind handles resource synchronization between consumer and provider clusters, with a focus on isolation strategies and namespace mapping.

This is development-focused documentation intended to help contributors understand the internal workings of kube-bind.
If you are looking for user-focused documentation, please refer to the [Synchronization User Guide](../usage/synchronization.md).

## Cluster-Scoped Resources

Cluster-scoped resources (like Sheriffs in our examples) can use different isolation strategies to control how they are placed on the provider cluster and how their associated secrets are isolated.

### Architecture Flow

```
CONSUMER CLUSTER                            PROVIDER CLUSTER (Backend)
================                            ==========================

┌─────────────────────────────┐           ┌────────────────────────────────────────────┐
│ Namespace: wild-west        │           │ Contract Namespace (per consumer):         │
│  - Secret: sheriff-badge-   │──────────▶│   kube-bind-<consumer-id-hash>             │
│    credentials (ref)        │   Sync    │                                            │
│  - Secret: sheriff-juris-   │           │   APIServiceNamespace CR:                  │
│    diction-config (label)   │           │     metadata.name: wild-west               │
└─────────────────────────────┘           │     status.namespace: (varies by isolation)│
                                          └────────────────────────────────────────────┘
┌─────────────────────────────┐
│ Sheriff: wyatt-earp         │──────────▶ PROVIDER CLUSTER (varies by isolation)
│  (cluster-scoped)           │   Sync    See isolation strategies below ↓
│  spec.intent                │
│  spec.secretRefs: [...]     │
└─────────────────────────────┘
         ▲
         │ Status updates
         └────────────────────────────────
```

### Isolation Strategies for Cluster-Scoped Resources

#### 1. IsolationPrefixed

```
┌────────────────────────────────────────────┐
│ [cc-prefixed] IsolationPrefixed:           │
│   Sheriff: <consumer-id>-wyatt-earp        │
│   (cluster-scoped, prefixed name)          │
│   Secrets in namespace:                    │
│     kube-bind-<consumer-id>-wild-west      │
│   (APIServiceNamespace mapping)            │
└────────────────────────────────────────────┘
```

**How it works:**
- Sheriff resource name is prefixed with consumer ID: `<consumer-id>-wyatt-earp`
- Sheriff remains cluster-scoped on provider
- Secrets are isolated via namespace mapping: `wild-west` → `kube-bind-<consumer-id>-wild-west`
- Multiple consumers can safely coexist with different prefixed resources

**Use case:** When you want cluster-scoped resources on the provider but need to support multiple consumers safely.

#### 2. IsolationNone

```
┌────────────────────────────────────────────┐
│ [cc-none] IsolationNone:                   │
│   Sheriff: wyatt-earp                      │
│   (cluster-scoped, same name on both!)     │
│   ⚠ Last write wins between consumers      │
│   Secrets in namespace: wild-west          │
│   (Same namespace name on both sides!)     │
└────────────────────────────────────────────┘
```

**How it works:**
- Sheriff resource keeps the same name on both sides: `wyatt-earp`
- Sheriff remains cluster-scoped on provider
- **Namespace mapping is 1:1**: `wild-west` → `wild-west` (same name!)
- **⚠️ WARNING:** No isolation! Multiple consumers share the same namespace. This is possible but discouraged.
- Last write wins if multiple consumers create the same resource

**Use case:** Single consumer scenarios or when you explicitly want shared resources. **Not recommended for multi-consumer environments.**

#### 3. IsolationNamespaced

```
┌────────────────────────────────────────────┐
│ [cc-namespaced] IsolationNamespaced:       │
│   Sheriff: wyatt-earp                      │
│   (CRD toggled to NamespaceScoped)         │
│   in namespace:                            │
│     kube-bind-<consumer-id>-wild-west      │
│   Secrets in same namespace (isolated)     │
└────────────────────────────────────────────┘
```

**How it works:**
- Sheriff CRD is toggled from ClusterScoped to NamespaceScoped on the provider
- Sheriff is placed in consumer-specific namespace: `kube-bind-<consumer-id>-wild-west`
- Secrets are in the same namespace (isolated)
- Namespace mapping: `wild-west` → `kube-bind-<consumer-id>-wild-west`

**Use case:** When you want the strongest isolation and are willing to modify the CRD scope on the provider.

### Key Insights for Cluster-Scoped Resources

The isolation strategy determines **BOTH** how Sheriff resources are placed **AND** how namespaces are mapped:

- **IsolationNone**: Same namespace name on both sides (`wild-west` → `wild-west`)
  - ⚠️ **NO isolation for secrets!** Multiple consumers can exist but share the same namespace (discouraged)

- **IsolationPrefixed**: Sheriff name prefixed, secrets isolated via namespace mapping
  - (`wild-west` → `kube-bind-<consumer-id>-wild-west`)

- **IsolationNamespaced**: Sheriff becomes namespaced, secrets isolated in same namespace
  - (`wild-west` → `kube-bind-<consumer-id>-wild-west`)

## Namespaced Resources

Namespaced resources (like Cowboys in our examples) always use the ServiceNamespaced isolation strategy, regardless of the isolation parameter.

### Architecture Flow

```
CONSUMER CLUSTER                            PROVIDER CLUSTER (Backend)
================                            ==========================

┌─────────────────────────────┐           ┌────────────────────────────────────────────┐
│ Namespace: wild-west        │           │ Contract Namespace (per consumer):         │
│                             │──────────▶│   kube-bind-<consumer-id-hash>             │
│  Cowboy: billy-the-kid      │   Sync    │                                            │
│   (namespaced)              │           │   APIServiceNamespace CR:                  │
│   spec.intent               │           │     metadata.name: wild-west               │
│   spec.secretRefs: [...]    │           │     status.namespace: ─────────────────┐   │
│                             │           │       kube-bind-<consumer-id>-wild-west│   │
│  - Secret: colt-45-permit   │           └────────────────────────────────────────┼───┘
│    (ref)                    │                                                    │
│  - Secret: cowboy-gang-     │           ACTUAL PROVIDER NAMESPACE: ◀─────────────┘
│    affiliation (label)      │           ┌────────────────────────────────────────────┐
└─────────────────────────────┘           │ Namespace: kube-bind-<consumer-id>-wild-   │
         ▲                                │            west                            │
         │ Status updates                 │                                            │
         └────────────────────────────────│  Cowboy: billy-the-kid (namespaced)        │
                                          │  - Secret: colt-45-permit                  │
                                          │  - Secret: cowboy-gang-affiliation         │
                                          │                                            │
                                          │ RBAC (for secret access):                  │
                                          │  - Role: kube-binder-export-wild-west-     │
                                          │    cowboys (in this namespace)             │
                                          │  - RoleBinding: ... (in this namespace)    │
                                          └────────────────────────────────────────────┘
```

### InformerScope Variations

Namespaced resources support two informer scope configurations:

#### 1. NamespacedScope (nn)

```
┌────────────────────────────────────────────────────────────────────┐
│ [nn] Namespaced → Namespaced (informerScope=NamespacedScope):     │
│   - Consumer: Cowboy in namespace wild-west                       │
│   - Provider: Cowboy in namespace kube-bind-<id>-wild-west        │
│   - Konnector watches only its own namespace on provider side     │
│   - Natural isolation via namespaces                              │
└────────────────────────────────────────────────────────────────────┘
```

**How it works:**
- Consumer resource is namespaced: `wild-west/billy-the-kid`
- Provider resource is namespaced: `kube-bind-<consumer-id>-wild-west/billy-the-kid`
- Konnector watches **only its own namespace** on the provider side
- RBAC uses namespace-scoped Roles and RoleBindings

**Use case:** Most efficient for namespaced resources, minimizes RBAC and watch scope.

#### 2. ClusterScope (nc)

```
┌────────────────────────────────────────────────────────────────────┐
│ [nc] Namespaced → Cluster (informerScope=ClusterScope):           │
│   - Consumer: Cowboy in namespace wild-west                       │
│   - Provider: Cowboy in namespace kube-bind-<id>-wild-west        │
│   - Konnector watches cluster-wide on provider side               │
│   - Still isolated via namespaces, but different RBAC model       │
└────────────────────────────────────────────────────────────────────┘
```

**How it works:**
- Consumer resource is namespaced: `wild-west/billy-the-kid`
- Provider resource is namespaced: `kube-bind-<consumer-id>-wild-west/billy-the-kid`
- Konnector watches **cluster-wide** on the provider side
- RBAC uses ClusterRoles and ClusterRoleBindings

**Use case:** When you need cluster-wide visibility or have specific RBAC requirements.

### Key Insights for Namespaced Resources

Namespaced resources **ALWAYS** use the ServiceNamespaced isolation strategy:

- The `isolation` parameter **only applies to cluster-scoped resources**
- Each consumer gets its own provider namespace via APIServiceNamespace mapping
- Namespace mapping is always: `wild-west` → `kube-bind-<consumer-id>-wild-west`
- The `informerScope` parameter only affects whether the konnector watches at namespace or cluster level
- Secrets are always isolated per consumer

## Complete Test Lifecycle

The end-to-end test validates the complete binding and synchronization flow:

### Setup Phase

1. **Create provider workspace (KCP)**
   - Install kube-bind CRDs
   - Start backend server (HTTP API for binding)
   - Bootstrap example CRDs (Cowboys/Sheriffs)
   - Apply templates (template-cowboys.yaml / template-sheriffs.yaml)

2. **Create consumer workspaces**
   - Start konnector on each consumer (watches APIServiceBinding)

### Binding Phase

3. **Login to provider**
   - Simulate browser auth flow
   - Save credentials to kube-bind-config.yaml

4. **List templates & collections**
   - Verify backend returns templates
   - Verify collections exist

5. **Bind API**
   - Send BindableResourcesRequest with templateRef and clusterIdentity
   - Backend creates on provider:
     - Contract namespace: `kube-bind-<consumer-id-hash>`
     - APIServiceExportRequest
     - APIServiceNamespace: `wild-west` → `kube-bind-<id>-wild-west`
     - RBAC resources for secret access
   - Backend returns BindingResourceResponse with CRDs, RBAC manifests

6. **Apply binding on consumer**
   - Create APIServiceBinding (watched by konnector)
   - Apply CRDs
   - Wait for CRD to be established

### Synchronization & Verification Phase

7. **Create instance on consumer**
   - Create resource (Cowboy or Sheriff) in namespace `wild-west`
   - Create associated secrets

8. **Verify sync to provider**
   - Instance created on provider with ClusterNamespaceAnnotation
   - Extract providerContractNamespace and providerObjectNamespace

9. **Verify namespace pre-seeding & RBAC**
   - APIServiceNamespace created with status.namespace populated
   - Actual provider namespace exists
   - RBAC resources created (ClusterRole/Role + Bindings)

10. **Test bi-directional sync**
    - Update spec on consumer → synced to provider
    - Update status on provider → synced to consumer

11. **Verify secret sync**
    - Referenced secret (spec.secretRefs) synced to provider namespace
    - Label-selected secret synced

12. **Test deletion**
    - Delete on consumer → deleted on provider

### Multi-Consumer Isolation Verification

Running with 2 consumers validates:
- Each consumer gets isolated contract namespace
- Each consumer gets isolated provider namespace (except with IsolationNone)
- Secrets are properly isolated (except with IsolationNone)
- For cluster-scoped resources with IsolationNone, last-write-wins
- For cluster-scoped resources with IsolationPrefixed, resources are name-prefixed
- For cluster-scoped resources with IsolationNamespaced, provider CRD is toggled to NamespaceScoped

## Implementation Details

### Code Structure

- **Isolation strategies**: `pkg/konnector/controllers/cluster/serviceexport/isolation/`
  - `none.go`: No isolation, same names/namespaces on both sides
  - `prefixed.go`: Prefix resource names with consumer ID
  - `namespaced.go`: Convert cluster-scoped to namespaced
  - `servicenamespaced.go`: APIServiceNamespace-based mapping for namespaced resources

- **Controller logic**: `pkg/konnector/controllers/cluster/serviceexport/serviceexport_reconcile.go`
  - Determines which isolation strategy to use based on resource scope and configuration
  - Lines 247-284: Strategy selection logic

### Strategy Selection Logic

```go
switch {
case schema.Spec.Scope == apiextensionsv1.NamespaceScoped:
    // ALWAYS use ServiceNamespaced for namespaced resources
    isolationStrategy = isolation.NewServiceNamespaced(...)

case export.Spec.Isolation == kubebindv1alpha2.IsolationNone:
    isolationStrategy = isolation.NewNone(...)

case export.Spec.Isolation == kubebindv1alpha2.IsolationPrefixed:
    isolationStrategy = isolation.NewPrefixed(...)

case export.Spec.Isolation == kubebindv1alpha2.IsolationNamespaced:
    isolationStrategy = isolation.NewNamespaced(...)
}
```

## References

- Test implementation: `test/e2e/bind/happy-case_test.go`
- API types: `sdk/apis/kubebind/v1alpha2/apiserviceexport_types.go`
- Isolation strategies: `pkg/konnector/controllers/cluster/serviceexport/isolation/`
