---
description: >
  How to setup a development environment for contributing to kube-bind.
title: kcp
---

# Development Environment using kcp

All the instructions assume you have already cloned the kube-bind repository and have Go installed.

kcp requires initial setup to be run before it can be used. This includes setting up workspace/provider
and setting up all the `APIResourceSchemas` and `APIExports`.

This is not required if you are doing deeper integration, and controlling the setup with your own scripts.

It's good to have the kcp CLI installed to help with workspace management:

```bash
kubectl krew index add kcp-dev https://github.com/kcp-dev/krew-index.git
kubectl krew install kcp-dev/kcp
kubectl krew install kcp-dev/ws
kubectl krew install kcp-dev/create-workspace
```

## Preparation

Start kcp:

```bash
make run-kcp
```

## Backend

### Bootstrap kcp

This is a dedicated step to set up kcp with required workspaces and `APIExports` for kube-bind:

```bash
cp .kcp/admin.kubeconfig .kcp/backend.kubeconfig
export KUBECONFIG=.kcp/backend.kubeconfig

./bin/kcp-init --kcp-kubeconfig $KUBECONFIG
```

### Run the Backend

```bash
kubectl ws use :root:kube-bind

go run ./cmd/backend \
  --multicluster-runtime-provider kcp \
  --oidc-issuer-url=http://127.0.0.1:8080/oidc \
  --oidc-callback-url=http://127.0.0.1:8080/api/callback \
  --oidc-type=embedded \
  --pretty-name="BigCorp.com" \
  --namespace-prefix="kube-bind-" \
  --schema-source apiresourceschemas \
  --consumer-scope=cluster
```

Optionally add `--fronend=http://localhost:3000` to point to local frontend (must be running
separately).

This process will keep running, so open a new terminal.

## Provider

### Kubeconfig Setup

Copy the kubeconfig to the provider and create provider workspace:

```bash
cp .kcp/admin.kubeconfig .kcp/provider.kubeconfig
export KUBECONFIG=.kcp/provider.kubeconfig
kubectl ws use :root
kubectl create-workspace provider --enter
```

### APIExport

Bind the APIExport to the provider workspace:

```bash
kubectl kcp bind apiexport root:kube-bind:kube-bind.io \
  --accept-permission-claim clusterrolebindings.rbac.authorization.k8s.io \
  --accept-permission-claim clusterroles.rbac.authorization.k8s.io \
  --accept-permission-claim customresourcedefinitions.apiextensions.k8s.io \
  --accept-permission-claim serviceaccounts.core \
  --accept-permission-claim configmaps.core \
  --accept-permission-claim secrets.core \
  --accept-permission-claim subjectaccessreviews.authorization.k8s.io \
  --accept-permission-claim namespaces.core \
  --accept-permission-claim roles.rbac.authorization.k8s.io \
  --accept-permission-claim rolebindings.rbac.authorization.k8s.io \
  --accept-permission-claim apiresourceschemas.apis.kcp.io
```

### Create CRD in provider

```bash
kubectl apply -f contrib/kcp/deploy/examples/apiexport.yaml
kubectl apply -f contrib/kcp/deploy/examples/apiresourceschema-cowboys.yaml
kubectl apply -f contrib/kcp/deploy/examples/apiresourceschema-sheriffs.yaml
kubectl kcp bind apiexport root:provider:cowboys-stable

kubectl apply -f deploy/examples/template-cowboys.yaml
kubectl apply -f deploy/examples/template-sheriffs.yaml
kubectl apply -f deploy/examples/collection.yaml
```

### Retrieve the LogicalCluster

```bash
kubectl get logicalcluster
# NAME      PHASE   URL
# cluster   Ready   https://192.168.2.166:6443/clusters/2myqz7lt9i0u5kzb
```

## Consumer

### Initialization

Now we gonna initiate consumer:

```bash
cp .kcp/admin.kubeconfig .kcp/consumer.kubeconfig
export KUBECONFIG=.kcp/consumer.kubeconfig

kubectl ws use :root
kubectl ws create consumer --enter
```

### Binding

Bind the thing:

```bash
./bin/kubectl-bind login http://127.0.0.1:8080 --cluster 1nso4d2rvleempdp
./bin/kubectl-bind --dry-run --cluster-identity-namespace default -o yaml > apiserviceexport.yaml

# Extract secret for binding process. Note that secret name is not the same as output from command above. Check secret
# name by running `kubectl get secret -n kube-bind`
kubectl get secrets -n kube-bind -o jsonpath='{.items[0].data.kubeconfig}' | base64 -d > remote.kubeconfig

namespace=$(yq '.contexts[0].context.namespace' remote.kubeconfig)

./bin/kubectl-bind apiservice \
  --remote-kubeconfig remote.kubeconfig \
  --remote-namespace "$namespace" \
  --skip-konnector \
  -f apiserviceexport.yaml \
  -v 6
```

This will keep running, so switch to a new terminal.

### Launch Konnector

Start konnector:

```bash
./bin/konnector --lease-namespace default --kubeconfig .kcp/consumer.kubeconfig --server-address ":9090"
```

Create example resources in consumer:

```bash
kubectl apply -f deploy/examples/cr-cowboy.yaml
kubectl apply -f deploy/examples/cr-sheriff.yaml
```
