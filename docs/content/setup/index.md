# Setting Up kube-bind

kube-bind supports multiple deployment scenarios and backend providers to meet different requirements.

## Setup Options

### Standard Kubernetes Setup

- **[Quickstart](quickstart.md)**: Get started quickly with the default provider
- **[Helm Deployment](helm.md)**: Production deployment using Helm charts
- **[Local Setup with Kind](local-setup-with-kind.md)**: Local development environment
- **[kubectl Plugin](kubectl-plugin.md)**: Install and use the kubectl-bind plugin

### Advanced Multi-Cluster Setup

- **[KCP Integration](kcp-setup.md)**: Advanced multi-tenant setup with kcp workspaces and APIExports

## Architecture Overview

Starting with v0.5.0, kube-bind uses a multicluster-runtime architecture that supports:

- **Multiple Providers**: Choose between standard Kubernetes or KCP backends
- **Enhanced API**: v1alpha2 API with resource-based exports and BoundSchema support
- **Flexible Deployment**: Support for various cluster topologies and requirements

Choose the setup that best fits your use case:

- Use **Quickstart** or **Local Setup with Kind** for development and testing
- Use **Helm Deployment** for production environments with standard Kubernetes
- Use **KCP Integration** for advanced multi-tenant scenarios with workspace isolation

## Next Steps

After completing your setup, explore these guides:

- **[Usage Guide](../usage/index.md)**: Learn common workflows and the new Catalog API
- **[Migration Guide](../usage/migration.md)**: Upgrade from previous versions
- **[Developer Documentation](../developers/index.md)**: Understand the architecture and contribute

{% include "partials/section-overview.html" %}
