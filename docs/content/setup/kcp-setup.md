---
description: >
  Set up kube-bind with KCP provider for advanced multi-cluster scenarios.
---

# kcp Setup

This guide shows how to set up kube-bind with the kcp provider, which enables advanced multi-cluster and multi-tenant scenarios through kcp workspaces.

## Overview

The kcp provider integrates kube-bind with [kcp](https://github.com/kcp-dev/kcp), enabling:

- **Workspace-based isolation**: Each binding can operate in isolated kcp workspaces
- **Advanced multi-tenancy**: Provider and consumer separation through logical clusters where backend can be in single workspace or multiple workspaces
- **APIExport integration**: Leverages kcp's APIExport mechanism for service exposure
- **Scalable architecture**: Supports large-scale multi-cluster deployments

## Prerequisites

- [kcp](https://github.com/kcp-dev/kcp) instance running and accessible
- [dex](https://github.com/dexidp/dex) for OIDC authentication
- kube-bind binaries built (`make build`)

## Setup Steps

### 1. Start KCP

```bash
make run-kcp
```

### 2. Start Dex OIDC Provider

Clone and configure dex:

```bash
git clone https://github.com/dexidp/dex.git
cd dex && make build
```

Configure dex (`examples/config-dev.yaml`):

```yaml
staticClients:
- id: kube-bind
  redirectURIs:
  - 'http://127.0.0.1:8080/callback'
  name: 'Kube Bind'
  secret: ZXhhbXBsZS1hcHAtc2VjcmV0
```

Start dex:

```bash
./bin/dex serve examples/config-dev.yaml
```

### 3. Bootstrap KCP

Create the kube-bind provider workspace and APIExport:

```bash
cp .kcp/admin.kubeconfig .kcp/backend.kubeconfig
export KUBECONFIG=.kcp/backend.kubeconfig
./bin/kcp-init --kcp-kubeconfig $KUBECONFIG
```

### 4. Start Backend with KCP Provider

Switch to the kube-bind workspace:

```bash
kubectl ws use :root:kube-bind
```

Start the backend with KCP provider:

```bash
./bin/backend \
  --multicluster-runtime-provider kcp \
  --server-url=$(kubectl get apiexportendpointslice kube-bind.io -o jsonpath="{.status.endpoints[0].url}") \
  --oidc-issuer-client-secret=ZXhhbXBsZS1hcHAtc2VjcmV0 \
  --oidc-issuer-client-id=kube-bind \
  --oidc-issuer-url=http://127.0.0.1:5556/dex \
  --oidc-callback-url=http://127.0.0.1:8080/callback \
  --pretty-name="BigCorp.com" \
  --namespace-prefix="kube-bind-" \
  --cookie-signing-key=bGMHz7SR9XcI9JdDB68VmjQErrjbrAR9JdVqjAOKHzE= \
  --cookie-encryption-key=wadqi4u+w0bqnSrVFtM38Pz2ykYVIeeadhzT34XlC1Y= \
  --schema-source apiresourceschemas
```

### 5. Create Provider Workspace

Create a provider workspace for hosting services:

```bash
cp .kcp/admin.kubeconfig .kcp/provider.kubeconfig
export KUBECONFIG=.kcp/provider.kubeconfig
kubectl ws use :root
kubectl ws create provider --enter
```

### 6. Bind APIExport to Provider

Bind the kube-bind APIExport to the provider workspace:

```bash
kubectl kcp bind apiexport root:kube-bind:kube-bind.io \
  --accept-permission-claim clusterrolebindings.rbac.authorization.k8s.io \
  --accept-permission-claim clusterroles.rbac.authorization.k8s.io \
  --accept-permission-claim customresourcedefinitions.apiextensions.k8s.io \
  --accept-permission-claim serviceaccounts.core \
  --accept-permission-claim configmaps.core \
  --accept-permission-claim secrets.core \
  --accept-permission-claim namespaces.core \
  --accept-permission-claim roles.rbac.authorization.k8s.io \
  --accept-permission-claim rolebindings.rbac.authorization.k8s.io \
  --accept-permission-claim apiresourceschemas.apis.kcp.io
```

### 7. Create Example Resources

Deploy example APIExport and APIResourceSchemas:

```bash
kubectl create -f contrib/kcp/deploy/examples/apiexport.yaml
kubectl create -f contrib/kcp/deploy/examples/apiresourceschema-cowboys.yaml
kubectl create -f contrib/kcp/deploy/examples/apiresourceschema-sheriffs.yaml

# Enable recursive binding
kubectl kcp bind apiexport root:provider:cowboys-stable
```

### 8. Get Logical Cluster Information

Retrieve the logical cluster URL for consumer setup:

```bash
kubectl get logicalcluster
# NAME      PHASE   URL                                                    AGE
# cluster   Ready   https://192.168.2.166:6443/clusters/2xh2v3gzjhn4tmve
```

## Consumer Setup

### 1. Create Consumer Workspace

```bash
cp .kcp/admin.kubeconfig .kcp/consumer.kubeconfig
export KUBECONFIG=.kcp/consumer.kubeconfig
kubectl ws use :root
kubectl ws create consumer --enter
```

### 2. Perform Binding

Generate the APIServiceExport YAML:

```bash
./bin/kubectl-bind http://127.0.0.1:8080/clusters/<logical-cluster-id>/exports --dry-run -o yaml > apiserviceexport.yaml
```

Extract the kubeconfig for binding:

```bash
kubectl get secret <secret-name> -n kube-bind -o jsonpath='{.data.kubeconfig}' | base64 -d > remote.kubeconfig
```

Perform the binding:

```bash
./bin/kubectl-bind apiservice \
  --remote-kubeconfig remote.kubeconfig \
  -f apiserviceexport.yaml \
  --skip-konnector \
  --remote-namespace kube-bind-<random-suffix>
```

### 3. Start Konnector

```bash
export KUBECONFIG=.kcp/consumer.kubeconfig
go run ./cmd/konnector/ --lease-namespace default
```

### 4. Test the Setup

Create example resources:

```bash
kubectl apply -f contrib/kcp/deploy/examples/cowboy.yaml
```

## Advanced Features

### Multiple Consumers

You can create multiple consumer workspaces to test multi-tenant scenarios:

```bash
# Create second consumer
cp .kcp/admin.kubeconfig .kcp/consumer2.kubeconfig
export KUBECONFIG=.kcp/consumer2.kubeconfig
kubectl ws use :root
kubectl ws create consumer2 --enter

# Repeat binding process with different namespace
# Start konnector on different port
go run ./cmd/konnector/ --lease-namespace default --server-address :8091
```

### Debugging

To debug the setup, use the following commands:

```bash
# Switch to debug workspace
cp .kcp/admin.kubeconfig .kcp/debug.kubeconfig
export KUBECONFIG=.kcp/debug.kubeconfig
kubectl ws use :root:kube-bind

# Check available resources
kubectl-s "$(kubectl get apiexportendpointslice kube-bind.io -o jsonpath="{.status.endpoints[0].url}")/clusters/*" api-resources

# List CRDs
kubectl-s "$(kubectl get apiexportendpointslice kube-bind.io -o jsonpath="{.status.endpoints[0].url}")/clusters/*" get crd
```

## Key Differences from Standard Setup

- **Provider Selection**: Uses `--multicluster-runtime-provider kcp` flag
- **Workspace Management**: Requires kcp workspace creation and management
- **APIExport Integration**: Leverages kcp's APIExport mechanism to enable shared backed service.
- **URL Structure**: Uses kcp-specific URLs with cluster identifiers. In production, this should be abstracted by a service wrapper.
- **Advanced Isolation**: Provides workspace-level isolation beyond namespaces