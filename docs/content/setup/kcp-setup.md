---
description: >
  Set up kube-bind with kcp provider for advanced multi-cluster scenarios.
---

# kcp Setup

This guide shows how to set up kube-bind with the kcp provider, which enables advanced multi-cluster and multi-tenant scenarios through kcp workspaces.

## Overview

The kcp provider integrates kube-bind with [kcp](https://github.com/kcp-dev/kcp), enabling:

- **Workspace-based isolation**: Each binding can operate in isolated kcp workspaces
- **Advanced multi-tenancy**: Provider and consumer separation through logical clusters where the backend can be in a single workspace or multiple workspaces
- **APIExport integration**: Leverages kcp's APIExport mechanism for service exposure
- **Scalable architecture**: Supports large-scale multi-cluster deployments

## Prerequisites

- [kcp](https://github.com/kcp-dev/kcp) instance running and accessible
- [dex](https://github.com/dexidp/dex) for OIDC authentication
- kube-bind binaries built (`make build`)

## Setup Steps

### 1. Start kcp

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

### 3. Bootstrap kcp

Create the kube-bind provider workspace and APIExport:

```bash
cp .kcp/admin.kubeconfig .kcp/backend.kubeconfig
export KUBECONFIG=.kcp/backend.kubeconfig
./bin/kcp-init --kcp-kubeconfig $KUBECONFIG
```

### 4. Start Backend with kcp Provider

Switch to the kube-bind workspace:

```bash
kubectl ws use :root:kube-bind
```

Start the backend with kcp provider:

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
  --consumer-scope=cluster
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

# Create templates and catalog
kubectl create -f contrib/kcp/deploy/examples/template-cowboys.yaml
kubectl create -f contrib/kcp/deploy/examples/template-sheriffs.yaml
kubectl create -f contrib/kcp/deploy/examples/collection-wildwest.yaml
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
./bin/kubectl-bind login http://127.0.0.1:8080 --cluster <logical-cluster-id>
./bin/kubectl-bind --skip-konnector
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
