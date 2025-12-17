---
description: >
  Set up kube-bind with kcp provider for advanced multi-cluster scenarios.
---

# kcp Setup

This guide shows how to set up kube-bind with the kcp provider where workspaces are treated as separate clusters.

## Prerequisites

- [kcp](https://github.com/kcp-dev/kcp) instance running and accessible
- kube-bind binaries built (`make build`)

## Setup Steps

1. Start kcp

```bash
make run-kcp
```

## Backend

2. Bootstrap backend workspace:
```bash
cp .kcp/admin.kubeconfig .kcp/provider.kubeconfig
export KUBECONFIG=.kcp/provider.kubeconfig
kubectl ws create provider --enter

kubectl apply -f deploy/crd
```

Apply example manifests:

```bash
kubectl apply -f deploy/examples/crd-cowboys.yaml
kubectl apply -f deploy/examples/crd-sheriffs.yaml
kubectl apply -f deploy/examples/collection.yaml
kubectl apply -f deploy/examples/template-cowboys.yaml
kubectl apply -f deploy/examples/template-sheriffs.yaml
```


3. Run the backend:
```
make build

bin/backend \
  --oidc-issuer-url=http://127.0.0.1:8080/oidc \
  --oidc-callback-url=http://127.0.0.1:8080/api/callback \
  --oidc-type=embedded \
  --pretty-name="BigCorp.com" \
  --namespace-prefix="kube-bind-" \
  --consumer-scope=cluster --frontend http://localhost:3000
```

This process will keep running, so open a new terminal.

## Consumer

4. Copy the kubeconfig to the provider and create provider workspace:
```bash
export KUBECONFIG=.kcp/admin.kubeconfig
kubectl ws create consumer --enter
```

5. Login into the backend and bind:

```bash
./bin/kubectl-bind login http://127.0.0.1:8080

./bin/kubectl-bind --dry-run -o yaml > apiserviceexport.yaml

# Extract secret for binding process. Note that secret name is not the same as output from command above. Check secret
# name by running `kubectl get secret -n kube-bind`
kubectl get secrets -n kube-bind -o jsonpath='{.items[0].data.kubeconfig}' | base64 -d > remote.kubeconfig

namespace=$(yq '.contexts[0].context.namespace' remote.kubeconfig)

./bin/kubectl-bind apiservice --remote-kubeconfig remote.kubeconfig -f apiserviceexport.yaml  --skip-konnector --remote-namespace $namespace

# run konnector in different terminal
export KUBECONFIG=.kcp/admin.kubeconfig
go run ./cmd/konnector/ --lease-namespace default
```

# Create an instance 

```bash
kubectl apply -f deploy/examples/cr-cowboy.yaml
kubectl apply -f deploy/examples/cr-sheriff.yaml
```
