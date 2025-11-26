---
title: Usage Guide
description: |
  Comprehensive guide to using kube-bind APIs, concepts, and workflows for service providers and consumers.
weight: 200
---

# kube-bind Usage Guide

This section provides comprehensive documentation on how to use kube-bind's core APIs and concepts. Whether you're a service provider looking to export APIs or a consumer wanting to bind to services, this guide covers the essential workflows and components.

## Core Concepts

kube-bind operates on three fundamental concepts:

### Service Provider

The cluster that **exports** APIs and resources, making them available for other clusters to consume. Service providers create templates and handle permission claims.

### Service Consumer

The cluster that **imports** and uses APIs from service providers. Consumers bind to templates and get access to resources through a secure, controlled process.

### Konnector Agent

The component that establishes and maintains the secure connection between provider and consumer clusters, synchronizing resources and handling permissions.

## Key API Types

### APIServiceExportTemplate

**Purpose**: Defines a reusable service template that groups related CRDs and permission claims.
**Used by**: Service providers
**Scope**: Template definition for multiple consumers

### APIServiceExport

**Purpose**: Represents an active export of a specific CRD to consumer clusters.
**Used by**: Automatically created by konnector agents
**Scope**: Per-CRD export instance

### APIServiceExportRequest

**Purpose**: Consumer's request to bind to a specific service template.
**Used by**: Service consumers (via CLI/UI)
**Scope**: Per-binding request

### APIServiceNamespace

**Purpose**: Manages namespace mapping and isolation between provider and consumer clusters.
**Used by**: Automatically managed by konnector agents
**Scope**: Per-namespace sync

## Documentation Structure

### [API Concepts](api-concepts.md)

Deep dive into the core API types, their relationships, and how they work together in the kube-bind ecosystem.

### [Template References](template-references.md)

Advanced guide for using dynamic resource selection through references in templates.

## Common Workflows

### For Service Providers

1. **Create templates** defining what APIs and resources to export, including permission claims
2. **Implement service** to act on the synced/bound objects so it can be returned to the consumer/user.

### For Service Consumers

1. **Authenticate** to the kube-bind backend
1. **Discover available templates** through the web UI or CLI
2. **Request bindings** to specific templates
3. **Authenticate and authorize** access through OAuth2 flows
4. **Use imported APIs** in their local cluster

### For Platform Operators

1. **Deploy kube-bind infrastructure** on both provider and consumer sides (if using GitOps)
2. **Configure authentication** and security policies
3. **Monitor connections** and resource synchronization

## Getting Started

If you're new to kube-bind:

1. **Start with the [Quickstart Guide](../setup/quickstart.md)** for a hands-on introduction
2. **Review [API Concepts](api-concepts.md)** to understand the fundamental types
3. **Explore [Template References](template-references.md)** for advanced use cases
4. **Check the [Reference Documentation](../reference/)** for complete API specifications

The konnector agents establish a secure, authenticated connection that allows:

- **API schema synchronization** from provider to consumer
- **Resource data flow** based on permission claims
- **Namespace isolation** and mapping
- **Authentication and authorization** enforcement
