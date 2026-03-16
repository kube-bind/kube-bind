# kcp

kcp folder contains isolated set of tooling to bootstrap kube-bind to work with a kcp instance.
It is split into a separate package to avoid vendoring pollution.

kcp requires initial setup before it can be used.
This includes setting up workspaces, `APIResourceSchema`s, and `APIExport`s.

It was its own Go module to avoid kcp dependencies in the main kube-bind module.

This is not required if you are doing deeper integration and controlling the setup with your own scripts.

## Architecture

This example demonstrates a **three-tier** kube-bind setup with kcp:

```
:root:provider   — owns the Cowboys APIResourceSchema + APIExport (source of truth)
      |
      |  (APIBinding: :root:backend binds cowboys from :root:provider)
      v
:root:backend    — binds cowboys from provider, re-exposes them to consumers via kube-bind
      |
      |  (kube-bind runs in :root:kube-bind, serves the binding API for consumers)
      v
  consumer       — any kcp workspace or external cluster that wants cowboys
```

Three kubeconfigs are used throughout this guide:

| File | Points to | Used for |
|------|-----------|----------|
| `provider.kubeconfig` | `:root:provider` | creating cowboys `APIResourceSchema` + `APIExport` |
| `backend.kubeconfig` | `:root:backend` | binding cowboys from provider, creating templates |
| `kube-bind.kubeconfig` | `:root:kube-bind` | running the kube-bind backend process |

- **Provider** (`:root:provider`): defines the `cowboys` API (`APIResourceSchema` + `APIExport`). The upstream API owner.
- **Backend** (`:root:backend`): binds provider's `cowboys` APIExport, creates `APIServiceExport` templates. The broker layer.
- **kube-bind** (`:root:kube-bind`): where the kube-bind backend process runs, serving the binding HTTP API to consumers.
- **Consumer**: binds cowboys via kube-bind, with no direct dependency on the provider.

## How to run

### Preparation

1. Start kcp:

```bash
make run-kcp
```

---

### Provider

The provider workspace owns the `cowboys` `APIResourceSchema` and `APIExport`.

2. Set up the provider workspace:


```bash
cp .kcp/admin.kubeconfig .kcp/provider.kubeconfig
export KUBECONFIG=.kcp/provider.kubeconfig
kubectl ws use :root
kubectl ws create provider --enter
```

3. Create the `APIResourceSchema` and `APIExport` for cowboys:

```bash
kubectl apply -f contrib/kcp/deploy/examples/apiresourceschema-cowboys.yaml
kubectl apply -f contrib/kcp/deploy/examples/apiexport-cowboys.yaml
```

The `APIExport` named `cowboys` makes the cowboys API available for other workspaces to bind.

---

### kube-bind

The `:root:kube-bind` workspace is where the kube-bind backend process runs. `kcp-init` creates this workspace and installs the kube-bind `APIExport` into it.

4. Bootstrap the kube-bind workspace:

```bash
cp .kcp/admin.kubeconfig .kcp/kube-bind.kubeconfig
export KUBECONFIG=.kcp/kube-bind.kubeconfig
./bin/kcp-init --kcp-kubeconfig $KUBECONFIG
```

5. Run the kube-bind backend:

```bash
kubectl ws use :root:kube-bind

go run ./cmd/backend \
  --multicluster-runtime-provider kcp \
  --apiexport-endpoint-slice-name=kube-bind.io \
  --pretty-name="BigCorp.com" \
  --oidc-type=embedded \
  --oidc-issuer-url=http://127.0.0.1:8080/oidc \
  --oidc-callback-url=http://127.0.0.1:8080/api/callback \
  --namespace-prefix="kube-bind-" \
  --schema-source apiresourceschemas \
  --consumer-scope=cluster
```

This process keeps running — open a new terminal.

---

### Backend

The backend workspace (`:root:backend`) binds the provider's cowboys `APIExport` and sets up `APIServiceExport` templates so kube-bind knows what to offer consumers.

6. Set up the backend workspace and bind the kube-bind `APIExport`:

```bash
cp .kcp/admin.kubeconfig .kcp/backend.kubeconfig
export KUBECONFIG=.kcp/backend.kubeconfig
kubectl ws use :root
kubectl ws create backend --enter

kubectl kcp bind apiexport root:kube-bind:kube-bind.io \
  --accept-permission-claim clusterrolebindings.rbac.authorization.k8s.io \
  --accept-permission-claim clusterroles.rbac.authorization.k8s.io \
  --accept-permission-claim customresourcedefinitions.apiextensions.k8s.io \
  --accept-permission-claim serviceaccounts.core \
  --accept-permission-claim configmaps.core \
  --accept-permission-claim secrets.core \
  --accept-permission-claim namespaces.core \
  --accept-permission-claim roles.rbac.authorization.k8s.io \
  --accept-permission-claim subjectaccessreviews.authorization.k8s.io \
  --accept-permission-claim rolebindings.rbac.authorization.k8s.io \
  --accept-permission-claim apiresourceschemas.apis.kcp.io \
  --accept-permission-claim apibindings.apis.kcp.io
```

7. Bind the provider's cowboys `APIExport` into `:root:backend`:

```bash
kubectl kcp bind apiexport root:provider:cowboys
```

9. Get the LogicalCluster identity (needed for consumer binding):

```bash
kubectl get logicalcluster
# NAME      PHASE   URL                                                    AGE
# cluster   Ready   https://192.168.2.166:6443/clusters/2ocmmccjkme8bof4
```

---

### Consumer

The consumer binds cowboys from kube-bind (backed by `:root:backend`), with no direct dependency on the provider.

10. Set up the consumer workspace:

```bash
cp .kcp/admin.kubeconfig .kcp/consumer.kubeconfig
export KUBECONFIG=.kcp/consumer.kubeconfig
kubectl ws use :root
kubectl ws create consumer --enter
```

// TODO: Backend in kcp should dynamically lifecycle schemas.

11. Log in and bind cowboys:

```bash
./bin/kubectl-bind login http://127.0.0.1:8080 --cluster 216gnjm1cqct4x1j

./bin/kubectl-bind --dry-run -o yaml > apiserviceexport.yaml

# Extract the secret for the binding process.
# Note: the secret name may differ — check with `kubectl get secret -n kube-bind`
kubectl get secrets -n kube-bind -o jsonpath='{.items[0].data.kubeconfig}' | base64 -d > remote.kubeconfig

namespace=$(yq '.contexts[0].context.namespace' remote.kubeconfig)

./bin/kubectl-bind apiservice -v 6 \
  --remote-kubeconfig remote.kubeconfig \
  -f apiserviceexport.yaml \
  --skip-konnector \
  --remote-namespace "$namespace"
```

This process keeps running — switch to a new terminal.

12. Start the konnector:

```bash
./bin/konnector --lease-namespace default --kubeconfig .kcp/consumer.kubeconfig
```

---

### Create an instance

Once binding is complete, create a cowboy on the consumer:

```bash
kubectl apply -f deploy/examples/cr-cowboy.yaml
```

The cowboy is synced to `:root:backend`, which in turn stores it via the provider's APIExport in `:root:provider`.

---

## Backend-only mode

Sometimes you want to run kube-bind without any frontend, HTTP API, or OIDC — useful in multi-tenant environments, GitOps pipelines, or when each tenant has their own identity provider.

The same three kubeconfigs apply:

| File | Points to | Used for |
|------|-----------|----------|
| `provider.kubeconfig` | `:root:provider` | creating cowboys `APIResourceSchema` + `APIExport` |
| `backend.kubeconfig` | `:root:backend` | binding cowboys from provider, creating templates |
| `kube-bind.kubeconfig` | `:root:kube-bind` | running the kube-bind backend process |

### Preparation

1. Start kcp:

```bash
make run-kcp
```

### Provider

2. Set up the provider workspace and cowboys API:

```bash
cp .kcp/admin.kubeconfig .kcp/provider.kubeconfig
export KUBECONFIG=.kcp/provider.kubeconfig
kubectl ws use :root
kubectl ws create provider --enter
kubectl apply -f contrib/kcp/deploy/examples/apiresourceschema-cowboys.yaml
kubectl apply -f contrib/kcp/deploy/examples/apiexport-cowboys.yaml
```

### kube-bind

3. Bootstrap the kube-bind workspace:

```bash
cp .kcp/admin.kubeconfig .kcp/kube-bind.kubeconfig
export KUBECONFIG=.kcp/kube-bind.kubeconfig
./bin/kcp-init --kcp-kubeconfig $KUBECONFIG
```

4. Run the backend in backend-only mode (no OIDC, no HTTP frontend):

```bash
kubectl ws use :root:kube-bind

go run ./cmd/backend \
  --multicluster-runtime-provider kcp \
  --apiexport-endpoint-slice-name=kube-bind.io \
  --pretty-name="BigCorp.com" \
  --frontend-disabled=true \
  --namespace-prefix="kube-bind-" \
  --schema-source apiresourceschemas \
  --consumer-scope=cluster \
  --isolation=None
```

This process keeps running — open a new terminal.

### Backend

5. Set up the backend workspace:

```bash
cp .kcp/admin.kubeconfig .kcp/backend.kubeconfig
export KUBECONFIG=.kcp/backend.kubeconfig
kubectl ws use :root
kubectl ws create backend --enter

kubectl kcp bind apiexport root:kube-bind:kube-bind.io \
  --accept-permission-claim clusterrolebindings.rbac.authorization.k8s.io \
  --accept-permission-claim clusterroles.rbac.authorization.k8s.io \
  --accept-permission-claim customresourcedefinitions.apiextensions.k8s.io \
  --accept-permission-claim serviceaccounts.core \
  --accept-permission-claim configmaps.core \
  --accept-permission-claim secrets.core \
  --accept-permission-claim namespaces.core \
  --accept-permission-claim roles.rbac.authorization.k8s.io \
  --accept-permission-claim subjectaccessreviews.authorization.k8s.io \
  --accept-permission-claim rolebindings.rbac.authorization.k8s.io \
  --accept-permission-claim apiresourceschemas.apis.kcp.io \
  --accept-permission-claim apibindings.apis.kcp.io
```

6. Bind the provider's cowboys `APIExport` into `:root:backend`:

```bash
kubectl kcp bind apiexport root:provider:cowboys
```

7. Create the `APIResourceSchema` and `APIServiceExport` template for cowboys:

```bash
kubectl apply -f contrib/kcp/deploy/examples/apiresourceschema-cowboys.yaml
kubectl apply -f deploy/examples/template-cowboys.yaml
```

8. Get the LogicalCluster identity:

```bash
kubectl get logicalcluster
# NAME      PHASE   URL                                                    AGE
# cluster   Ready   https://192.168.2.166:6443/clusters/20nuv280snhqd5j4
```

### Consumer (backend-only)

9. Set up the consumer workspace:

```bash
cp .kcp/admin.kubeconfig .kcp/consumer.kubeconfig
export KUBECONFIG=.kcp/consumer.kubeconfig
kubectl ws use :root
kubectl ws create consumer --enter
```

In backend-only mode there is no HTTP API or OIDC, so the CLI login flow does not apply.
Instead, binding is initiated via a `BindableResourcesRequest` CRD.

**How backend-only binding works:**

In traditional mode, `BindableResourcesRequest` is sent as a REST API payload to the backend HTTP server, which:
1. Creates a dedicated namespace for the consumer
2. Generates a kubeconfig for that namespace
3. Returns it as an HTTP response

In backend-only mode, `BindableResourcesRequest` is a **CRD** watched by a controller, which:
1. Creates the namespace and kubeconfig automatically
2. Stores the result in a Secret specified by `kubeconfigSecretRef`

This enables GitOps and automation without requiring HTTP API access.

10. Get the cluster identity:

```bash
./bin/kubectl-bind cluster-identity
```

11. Create the `BindableResourcesRequest` (applied against `:root:kube-bind` where the backend is running):

```bash
export KUBECONFIG=.kcp/kube-bind.kubeconfig
kubectl ws use :root:kube-bind

kubectl apply -f - <<EOF
apiVersion: kube-bind.io/v1alpha2
kind: BindableResourcesRequest
metadata:
  name: 0ac6800e-bc4f-4c70-814b-45b44e04aa02
  namespace: default
spec:
  kubeconfigSecretRef:
    name: 0ac6800e-bc4f-4c70-814b-45b44e04aa02-response
    key: response
  author: "backend-only-user"
  clusterIdentity:
    identity: 0ac6800e-bc4f-4c70-814b-45b44e04aa02
EOF
```

12. Extract the binding response:

```bash
kubectl get secret 0ac6800e-bc4f-4c70-814b-45b44e04aa02-response \
  -o jsonpath='{.data.response}' | base64 -d > remote.data
```

13. Deploy the binding on the consumer:

```bash
export KUBECONFIG=.kcp/consumer.kubeconfig
./bin/kubectl-bind deploy --file remote.data --skip-konnector
```

14. Start the konnector:

```bash
./bin/konnector --lease-namespace default --kubeconfig .kcp/consumer.kubeconfig
```

15. Create an `APIServiceBindingBundle` to pull all available contracts:

```bash
kubectl apply -f - <<EOF
apiVersion: kube-bind.io/v1alpha2
kind: APIServiceBindingBundle
metadata:
  name: all-bindings
spec:
  kubeconfigSecretRef:
    key: kubeconfig
    name: kubeconfig-rxwnz
    namespace: kube-bind
EOF
```

### Approve exports on the backend

Binding is a two-way process: the backend must approve what gets exported to each consumer.

On `:root:backend`, create an `APIServiceExportRequest` for the consumer's namespace:

```bash
export KUBECONFIG=.kcp/backend.kubeconfig
kubectl ws use :root:backend

kubectl apply -n kube-bind-1iax4hdl6mmvc -f - <<EOF
apiVersion: kube-bind.io/v1alpha2
kind: APIServiceExportRequest
metadata:
  name: cowboys
spec:
  permissionClaims:
  - group: ""
    resource: secrets
    selector:
      labelSelector:
        matchLabels:
          app: cowboy
  resources:
  - group: wildwest.dev
    resource: cowboys
    versions:
    - v1alpha1
EOF
```

This automatically creates the required contracts and bindings on the consumer side.

### Create an instance (backend-only)

```bash
export KUBECONFIG=.kcp/consumer.kubeconfig
kubectl apply -f deploy/examples/cr-cowboy.yaml
```
