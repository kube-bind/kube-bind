---
title: API Concepts
description: |
  Deep dive into kube-bind's core API types, their relationships, and how they work together to enable cross-cluster service binding.
weight: 210
---

# kube-bind API Concepts

This guide provides a comprehensive overview of kube-bind's core API types and how they work together to enable secure, cross-cluster service binding.

## Overview

kube-bind uses several specialized Kubernetes custom resources to orchestrate the export and import of APIs between clusters. Understanding these types and their relationships is essential for effective use of kube-bind.

## APIServiceExportTemplate

**Purpose**: Defines a reusable template for exporting a group of related APIs and their permission requirements.

**Used by**: Service providers
**Scope**: Cluster-scoped
**Lifecycle**: Long-lived template definition

### Key Features

- **Resource Grouping**: Combines multiple related CRDs into a single bindable service
- **Permission Claims**: Specifies additional resources the service needs access to
- **Namespace Management**: Defines namespaces that should be created on the provider & consumer side
- **Reusability**: Can be bound by multiple consumers

### Structure

```yaml
apiVersion: kube-bind.io/v1alpha2
kind: APIServiceExportTemplate
metadata:
  name: my-service-template
spec:
  scope: Cluster # or Namespace
  description: "A comprehensive service template"

  # Core APIs being exported
  resources:
  - group: example.com
    resource: widgets
    versions: ["v1", "v1alpha1"]
  - group: example.com
    resource: gadgets
    versions: ["v1"]

  # Additional resources the service needs access to
  permissionClaims:
  - group: ""
    resource: secrets
    selector:
      labelSelector:
        matchLabels:
          component: my-service
      references:
      - resource: widgets
        group: example.com
        jsonPath:
          name: spec.secretRef.name
          namespace: spec.secretRef.namespace

  # Pre-created namespaces for this service. These will be owned by provider (service owned).
  namespaces:
  - name: my-service-system
    description: "System namespace for service components"
```

### Template vs Instance

- **Template**: The definition (APIServiceExportTemplate) - shared and reusable
- **Instance**: The actual export (APIServiceExport) - created per binding

## APIServiceExport

**Purpose**: Represents an active, instantiated export of a specific set of CRD's and permission claims to consumer clusters.

**Used by**: Automatically created by konnector agents
**Scope**: Namespaced
**Lifecycle**: Created when consumers bind to templates

### Key Features

- **Instance Management**: One APIServiceExport per contract per binding
- **Status Tracking**: Reports synchronization status and conditions
- **Automatic Lifecycle**: Created/updated/deleted by the konnector agent

### Structure

```yaml
apiVersion: kube-bind.io/v1alpha2
kind: APIServiceExport
metadata:
  name: widgets-export-consumer123
  namespace: kube-bind-provider-ns
spec:
  # The CRD being exported
  resources:
  - group: example.com
    resource: widgets
    versions: ["v1"]

  # Permission claims for this specific export
  permissionClaims:
  - group: ""
    resource: secrets
    selector:
      labelSelector:
        matchLabels:
          component: widget-service
          consumer: consumer123

  # How isolation is done at the provider side
  isolation: Prefixed
  # informerScope is the scope of the APIServiceExport. It can be either Cluster or Namespace.
	#
	# Cluster:    The konnector has permission to watch all namespaces at once and cluster-scoped resources.
	#             This is more efficient than watching each namespace individually.
	# Namespaced: The konnector has permission to watch only single namespaces.
	#             This is more resource intensive. And it means cluster-scoped resources cannot be exported.
  informerScope: Cluster

status:
  conditions:
  - lastTransitionTime: "2025-11-14T12:02:29Z"
    status: "True"
    type: Connected
  - lastTransitionTime: "2025-11-14T12:02:30Z"
    status: "True"
    type: ConsumerInSync
  - lastTransitionTime: "2025-11-14T12:02:25Z"
    status: "True"
    type: ProviderInSync
```

### Relationship to Templates

```
APIServiceExportTemplate (1)
    ↓ (consumer binding creates)
APIServiceExportRequest (1)
    ↓ (provider processes)
APIServiceExport + BoundSchema (1)
    ↕ (bidirectional sync)
APIServiceBinding (consumer side)
```

## APIServiceExportRequest

**Purpose**: Represents a consumer's request to bind to a specific service template.

**Used by**: Service consumers (via CLI/UI)
**Scope**: Namespaced (on consumer side)
**Lifecycle**: Created during binding process, short lived until APIServiceExport is established.

### Key Features

- **Binding Initiation**: Starts the binding process between consumer and provider
- **Authentication Context**: Contains OAuth2 flow details and credentials
- **Template Reference**: Points to the specific template being requested
- **Status Tracking**: Reports binding progress and any errors

### Structure

```yaml
apiVersion: kube-bind.io/v1alpha1
kind: APIServiceExportRequest
metadata:
  name: my-widget-service-binding
  namespace: default
spec:
  # Consumer-specific configuration. Matches template and export specification.
  resources:
  - group: example.com
    resource: widgets
    versions: ["v1"]

  permissionClaims:
  - group: core
    resource: configmaps
    # Selector for claimed resources. NamedResources, LabelSelector and References - OR'ed together.
    selector:
    # namedResources:
    # - name:
    #   namespace:
    # labelSelector:
    #  Standard label selector syntax
    # references:
    # - group:
    #   resource:
    #   versions: []
    #   jsonPath:
    #    name:
    #    namespace:


status:
  conditions:
  - lastTransitionTime: "2025-11-14T12:02:25Z"
    status: "True"
    type: Ready
  - lastTransitionTime: "2025-11-14T12:02:25Z"
    status: "True"
    type: ExportsReady
  phase: Succeeded
```

## APIServiceBinding

**Purpose**: Represents the consumer-side binding to a provider service, containing the applied CRDs and managing the resource synchronization.

**Used by**: Automatically created by consumer konnector agents
**Scope**: Namespaced (on consumer side)
**Lifecycle**: Created when APIServiceExportRequest is processed, long-lived

### Key Features

- **Resource Sync**: Manages bidirectional resource synchronization with provider
- **Connection State**: Tracks connection health and authentication status

### Structure

```yaml
apiVersion: kube-bind.io/v1alpha2
kind: APIServiceBinding
metadata:
  name: sheriffs
spec:
  # Secret for connecticity to the provider cluster.
  kubeconfigSecretRef:
    key: kubeconfig
    name: kubeconfig-xbm9b
    namespace: kube-bind
status:
  # boundSchemas are provider side schema tracking object, used by this connection/binding.
  boundSchemas:
  - group: wildwest.dev
    resource: sheriffs
  # other generic statuses & conditions
  conditions:
  - lastTransitionTime: "2025-11-18T07:16:52Z"
    status: "True"
    type: Ready
  - lastTransitionTime: "2025-11-18T07:16:52Z"
    status: "True"
    type: Connected
  - lastTransitionTime: "2025-11-18T07:16:52Z"
    status: "True"
    type: Heartbeating
  - lastTransitionTime: "2025-11-18T07:16:52Z"
    status: "True"
    type: InformersSynced
  - lastTransitionTime: "2025-11-18T07:16:52Z"
    status: "True"
    type: SchemaInSync
  - lastTransitionTime: "2025-11-18T07:16:52Z"
    status: "True"
    type: SecretValid
  # permissionClaims used by this connection/binding
  permissionClaims:
  - group: ""
    resource: secrets
    selector:
      labelSelector:
        matchLabels:
          app: sheriff
      references:
      - group: wildwest.dev
        jsonPath:
          name: spec.secretRefs.#.name
          namespace: spec.secretRefs.#.namespace
        resource: sheriffs
  providerPrettyName: BigCorp.com
```

## BoundSchema

**Purpose**: Contains the actual CRD definitions and schema information for resources bound from a provider.

**Used by**: Created alongside APIServiceExport on provider side
**Scope**: Namespaced (on provider side)
**Lifecycle**: Mirrors APIServiceExport lifecycle

### Key Features

- **Schema Storage**: Contains complete CRD definitions to sync to consumers
- **Version Management**: Handles multiple API versions and schema evolution
- **Validation Rules**: Includes OpenAPI schemas and validation logic
- **Resource Metadata**: Additional metadata about the exported resources

### Structure

```yaml
apiVersion: kube-bind.io/v1alpha2
kind: BoundSchema
metadata:
  name: widgets-schema-consumer123
  namespace: kube-bind-provider-ns
  ownerReferences:
  - apiVersion: kube-bind.io/v1alpha2
    kind: APIServiceExport
    name: widgets-export-consumer123
spec:
  group: wildwest.dev
  informerScope: Cluster
  names:
    kind: Sheriff
    listKind: SheriffList
    plural: sheriffs
    singular: sheriff
  scope: Cluster
  versions:
  - name: v1alpha1
    schema:
     # Complete OpenAPI schema definition
    served: true
    storage: true
    subresources:
      status: {}
          # Complete OpenAPI schema definition

status:
  acceptedNames:
    kind: Sheriff
    listKind: SheriffList
    plural: sheriffs
    singular: sheriff
  conditions:
  - lastTransitionTime: "2025-11-14T12:02:34Z"
    status: "True"
    type: Ready
  - lastTransitionTime: "2025-11-14T12:02:34Z"
    message: the initial names have been accepted
    reason: InitialNamesAccepted
    status: "True"
    type: Established
  - lastTransitionTime: "2025-11-14T12:02:29Z"
    message: no conflicts found
    reason: NoConflicts
    status: "True"
    type: NamesAccepted
```

## APIServiceNamespace

**Purpose**: Manages namespace mapping and isolation between provider and consumer clusters.

**Used by**: Automatically managed by konnector agents
**Scope**: Namespaced (on provider side)
**Lifecycle**: Created as needed during resource synchronization or by provider, when namespace is desired on consumer side.

### Key Features

- **Namespace Isolation**: Ensures consumer resources don't conflict
- **Mapping Logic**: Handles namespace translation between clusters
- **Resource Organization**: Groups related resources per consumer
- **Automatic Management**: Created/deleted as bindings change

## Complete Binding Flow

The correct flow from template to active binding:

### 1. Template Definition (Provider)
```yaml
APIServiceExportTemplate → defines service contract
```

### 2. Consumer Request
```yaml
APIServiceExportRequest → consumer requests binding
```

### 3. Provider Processing
```yaml
APIServiceExport + BoundSchema → provider creates export with schema
```

### 4. Consumer Binding
```yaml
APIServiceBinding → consumer applies CRDs and establishes sync
```

### 5. Bidirectional Sync
```yaml
Provider Resources ↔ Consumer Resources (via konnector agents)
```

### Structure

```yaml
apiVersion: kube-bind.io/v1alpha2
kind: APIServiceNamespace
metadata:
  # name presents namespace on consumer side
  name: wild-west
  # provider namespace, 1:1 mapped to consumer
  namespace: kube-bind-t5gf9
  ownerReferences:
  - apiVersion: kube-bind.io/v1alpha2
    blockOwnerDeletion: true
    controller: true
    kind: APIServiceExport
    name: sheriffs
    uid: fbd6c04e-e134-4156-89f4-4b31fea16657
spec: {}
status:
  # namespace, assigned on provider cluster
  namespace: kube-bind-t5gf9-wild-west
```

### Namespace Isolation Modes

kube-bind supports different isolation strategies:

- **Prefixed**: Consumer namespace becomes `{consumer-id}-{original-name}`
- **Namespaced**: Consumer resources go into dedicated provider namespaces
- **None**: For dedicated provider clusters where isolation isn't needed

## API Relationships and Data Flow

```
┌─────────────────────────────────────┐ Provider Cluster
│                                     │
│  APIServiceExportTemplate           │
│         │                           │
│         ▼ (processes request)       │
│  APIServiceExport ──► APIServiceNamespace
│         +                   │       │
│  BoundSchema                │       │
│         │                   │       │
│         ▼ (schema sync)     │       │
└─────────┼───────────────────┼───────┘
          │                   │
     [Secure Connection]      │
          │                   │
┌─────────▼───────────────────▼───────┐ Consumer Cluster
│                                     │
│  APIServiceExportRequest            │
│         │                           │
│         ▼ (creates binding)         │
│  APIServiceBinding ◄─── Applied CRDs│
│         │                   │       │
│         ▼                   │       │
│  Available APIs             │       │
│         │                   │       │
│         ▼ (resource sync)   │       │
│  Synchronized Resources ────┘       │
│                                     │
└─────────────────────────────────────┘
```

## Permission Claims Deep Dive

Permission claims are a critical concept that determines what additional resources a service can access beyond its own CRDs.

### Claim Types

#### Label Selector Claims
Select resources based on labels:
```yaml
permissionClaims:
- group: ""
  resource: secrets
  selector:
    labelSelector:
      matchLabels:
        component: my-service
        environment: production
```

#### Named Resource Claims
Select specific resources by name:
```yaml
permissionClaims:
- group: ""
  resource: configmaps
  selector:
    namedResources:
    - name: service-config
      namespace: kube-system
    - name: feature-flags
      namespace: default
```

#### Reference Claims (Dynamic)
Select resources based on references from other objects:
```yaml
permissionClaims:
- group: ""
  resource: secrets
  selector:
    references:
    - resource: widgets
      group: example.com
      jsonPath:
        name: spec.secretRef.name
        namespace: spec.secretRef.namespace
```

### Claim Evaluation

Permission claims are evaluated when:

1. **Template binding occurs** - Initial claim evaluation

2. **Reference sources change** - Dynamic re-evaluation for reference claims

3. **Resource labels change** - Re-evaluation for label selector claims

## Related Documentation

- [CRD Reference](../../reference/crd/kube-bind.io/) - Complete API specifications
- [CLI Reference](../../reference/) - Command-line tool documentation
