---
title: Resource Synchronization
description: |
  Overview over the general resource synchronization logic and different isolation modes.
weight: 220
---

# Resource Synchronization

This document describes the way kube-bind synchronizes Kubernetes resources across clusters.

## Overview

kube-bind synchronizes objects between two kinds of clusters:

* The **provider cluster** is run by a service provider and hosts a service, operator, or any other kind of Kubernetes API. This is also where there kube-bind **backend** is running in order to offer these APIs to consumers.
* The **consumer clusters** are where endusers consume services/APIs offered by providers.

There exists a 1:n relationship, where one provider cluster can be connected to from many different consumer clusters.

### Cluster Namespaces

In kube-bind, each successful `bind` operation yields a new, so-called "cluster namespace" on the provider cluster. This namespace contains all kube-bind-related resources, like `APIServiceExports` or `BoundSchemas` and is communicated to the consumer by being included in the provided kubeconfig.

By default the cluster namespaces are named `kube-bind-[random string]`, like `kube-bind-hd73s`. Their name also serves as a unique identifier for this "contract" between consumer and provider and is used in other places, for example as a prefix in the `prefixed` cluster isolation mode.

### Sync Direction

In the kube-bind architecture, the consumer clusters represent the source of truth (the actual desired state by the enduser) and the provider clusters merely contain copies of those objects.

During the synchronization,

* the **spec** (desired state) of an object is copied from the consumer cluster to the provider and
* the **status** (if any) is copied in the opposite direction, to the consumer.

kube-bind will continuously watch the object and its copy on both clusters and update the other side as needed.

### Connectivity

The object synchronization logic lives in the kube-bind **konnector**, a Kubernetes agent that runs on each *consumer cluster*. It reads a local Secret that contains a kubeconfig pointing to **a specific namespace** on the *provider cluster*. In this namespace the konnector will find all resources describing the resources to sync (chiefly `APIServiceExports`, `APIServiceNamespaces` and  `BoundSchemas`).

!!! note
    This kubeconfig is automatically generated as part of the `kubectl bind` handshake with a service provider.

This design allows consumer clusters to be mostly firewalled off, but requires provider clusters to not only be reachable from all consumers, but also to ensure multiple consumers do not conflict with each other. This is achieved by a combination of RBAC and isolation modes, which are described further down in this document.

### RBAC

Great care must be taken to ensure multiple consumers do not collide with each other on a single provider cluster. To achieve this, kube-bind offers two different informer scoping options:

* `namespaced` will make the konnector watch and inform on each relevant namespace on the provider cluster individually.
* `cluster` will instead make the konnector use a cluster-scoped (global) informer. This scoping method requires the konnector to have much wider permissions, but is more performant.

Regardless of scoping mode, the konnector itself will take care to not overwrite/touch other consumers' objects. This however does not mean that an attacker with access to the konnector kubeconfig could not exploit an RBAC policy that is too wide.

## Cluster Isolation

This section outlines how kube-bind deals with differently scoped Kubernetes objects.

In Kubernetes, an object can be either namespaced or cluster-scoped. This scope greatly affects how kube-bind processes the object.

### Namespaced

For namespaced objects (like a `Deployment`), kube-bind will map the consumer-side namespace to a unique, but random provider-side namespace: first the konnector will create an `APIServiceNamespace` object on the provider cluster (inside the cluster namespace), with the name of the consumer-side namespace (so a namespace `app1` will lead to an `APIServiceNamespace` `app1` in the cluster namespace). After this, the konnector waits until the backend has assigned this `APIServiceNamespace` a provider-side namespace to use. Once that namespace is present in the `APIServiceNamespace`'s status, the konnector will use it for all objects originating in the same consumer-side namespace.

In YAML, this means

```yaml
# consumer-side object

apiVersion: provider.example.com/v1
kind: MangoDB
metadata:
  name: my-first-db
  namespace: team1
spec:
  size: large
```

will lead to

```yaml
# provider-side objects

apiVersion: kube-bind.io/v1alpha2
kind: APIServiceNamespace
metadata:
  name: team1                # name of the namespace from the consumer
  namespace: kube-bind-hd73d # cluster namespace
spec: {}
status:
  # a common implementation in the backend is to construct the provider-side
  # namespace by just concatenating the two values, like so:
  namespace: kube-bind-hd73d-team1

---
apiVersion: v1
kind: Namespace
metadata:
  name: kube-bind-hd73d-team1

---
apiVersion: provider.example.com/v1
kind: MangoDB
metadata:
  name: my-first-db
  namespace: kube-bind-hd73d-team1
spec:
  size: large
```

!!! note
    For natively namespaced resources (those that are namespaced in both the provider and consumer cluster), this is always the strategy being used by the konnector. The other isolation modes only influence how the konnector deals with cluster-scoped objects in the consumer cluster.

### Cluster-Scoped

Cluster-scoped objects require a different approach to isolation than namespaced objects. kube-bind offers three different so-called isolation strategies to deal with them. The strategy to use is configured globally via the `--isolation` CLI flag on the kube-bind backend and will from there affect all services offered by that backend.

#### None Strategy

The `none` strategy provides no consumer-separation at all. Any cluster-scoped object on the consumer side is copied 1:1 to the provider side.

!!! warning
    Due to the obvious downsides of this approach, `none` should be used only in special circumstances.

#### Prefixed Strategy

The `prefixed` strategy is kube-bind's default strategy and will use the name of the cluster namespace as a prefix for object names.

In YAML, this means

```yaml
# consumer-side
apiVersion: provider.example.com/v1
kind: MangoDB
metadata:
  name: my-first-db
spec:
  size: large
```

will lead to

```yaml
# provider-side
apiVersion: provider.example.com/v1
kind: MangoDB
metadata:
  name: kube-bind-hd73d-my-first-db
spec:
  size: large
```

This strategy works well for separating objects, but

* requires a broad RBAC policy, which will always grant too many permissions, and
* would technically break for Kubernetes objects with names close to the maximum allowed length (253 characters).

#### Namespaced Strategy

!!! note
    Not to be confused with the strategy used for natively namespaced resources, described earlier.

The `namespaced` strategy will convert cluster-scoped objects into namespaced ones.

For this to work, the original CRD on the provider cluster has to be **namespaced**. The backend will turn it into a cluster-scoped CRD (stored in the `BoundSchema`), so on the consumer cluster, all objects are cluster-scoped.

During synchronization, the konnector will then place each cluster-scoped object into the cluster namespace (`kube-bind-hd73d`) on the provider side.

This strategy provides excellent isolation between consumers, but requires that the original CRD from the provider still makes sense to the consumer when it's suddenly cluster-scoped. For example, if references were to be used, especially to Secrets in the same namespace, this concept would get mangled to some degree during the synchronization.

!!! warning
    At the moment, when the backend is started with `--isolation=namespaced`, it will convert **all namespaced CRDs** in all ServiceExports to become cluster-scoped on the consumer side, even if you intended for a namespaced CRD to stay namespaced on both sides of the sync.
