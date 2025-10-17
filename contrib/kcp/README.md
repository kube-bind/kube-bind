# kcp

kcp folder contains isolated set of tooling to bootstrap the kube-bind to allow it to work with kcp instance.
It is split into separate package to avoid vendoring pollution.

kcp requires initial setup to be run before it can be used.
This includes setting up workspace/provider and setting up all the APIResourceSchemas and APIExports.

It was its own GO module to avoid kcp dependencies in the main kube-bind module.

This is not required if you are doing deeper integration, and controlling the setup with your own scripts.

It will do the following:
1. Create a provider workspace:
```
:root:kube-bind
```
2. Create apiexport inside the workspace:
```
:root:kube-bind/apiexport/kube-bind.io
```


# How to run

## Preparation

1. Start dex

```bash
make run-dex
```

2. Start kcp

```bash
make run-kcp
```

## Backend

2. Bootstrap kcp:
```bash
cp .kcp/admin.kubeconfig .kcp/backend.kubeconfig
export KUBECONFIG=.kcp/backend.kubeconfig
./bin/kcp-init --kcp-kubeconfig $KUBECONFIG
```
4. Run the backend:
```
k ws use :root:kube-bind

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
  --schema-source apiresourceschemas \
  --consumer-scope=cluster
```

This process will keep running, so open a new terminal.

## Provider

5. Copy the kubeconfig to the provider and create provider workspace:
```bash
cp .kcp/admin.kubeconfig .kcp/provider.kubeconfig
export KUBECONFIG=.kcp/provider.kubeconfig
k ws use :root
kubectl create-workspace provider --enter
```

6. Bind the APIExport to the provider workspace
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

7. Create CRD in provider:
```bash
kubectl create -f contrib/kcp/deploy/examples/apiexport.yaml
kubectl create -f contrib/kcp/deploy/examples/apiresourceschema-cowboys.yaml
kubectl create -f contrib/kcp/deploy/examples/apiresourceschema-sheriffs.yaml
kubectl kcp bind apiexport root:provider:cowboys-stable
```

8. Get LogicalCluster:

```bash
kubectl get logicalcluster
# NAME      PHASE   URL                                                    AGE
# cluster   Ready   https://192.168.2.166:6443/clusters/1d5vpxvdpy0opbj1
```

## Consumer

9. Now we gonna initiate consumer:
```bash
cp .kcp/admin.kubeconfig .kcp/consumer.kubeconfig
export KUBECONFIG=.kcp/consumer.kubeconfig
kubectl ws use :root
kubectl ws create consumer --enter
```

10. Bind the thing:

```bash
./bin/kubectl-bind http://127.0.0.1:8080/clusters/1d5vpxvdpy0opbj1/exports --dry-run -o yaml > apiserviceexport.yaml

# Extract secret for binding process. Note that secret name is not the same as output from command above. Check secret
# name by running `kubectl get secret -n kube-bind`
kubectl get secrets -n kube-bind -o jsonpath='{.items[0].data.kubeconfig}' | base64 -d > remote.kubeconfig

namespace=$(yq '.contexts[0].context.namespace' remote.kubeconfig)

./bin/kubectl-bind apiservice -v 6 --remote-kubeconfig remote.kubeconfig -f apiserviceexport.yaml  --skip-konnector --remote-namespace "$namespace"
```

This will keep running, so switch to a new terminal.

### Consumer Konnector

Start konnector:

```bash
./bin/konnector --lease-namespace default --kubeconfig .kcp/consumer.kubeconfig
```

Optionally add second consumer to test

```bash
cp .kcp/admin.kubeconfig .kcp/consumer2.kubeconfig
export KUBECONFIG=.kcp/consumer2.kubeconfig
kubectl ws use :root
kubectl ws create consumer2 --enter

./bin/kubectl-bind http://127.0.0.1:8080/clusters/2vgrh380y0cq38du/exports --dry-run -o yaml > apiserviceexport2.yaml

kubectl get secrets -n kube-bind -o jsonpath='{.items[0].data.kubeconfig}' | base64 -d > remote2.kubeconfig

./bin/kubectl-bind apiservice -v 6 --remote-kubeconfig remote2.kubeconfig -f apiserviceexport2.yaml  --skip-konnector --remote-namespace "$(yq '.contexts[0].context.namespace' remote2.kubeconfig)"

./bin/konnector --lease-namespace default --kubeconfig .kcp/consumer2.kubeconfig --server-address :8091
```

This will keep running, so switch to a new terminal.

## Testing

Create objects:
```
export KUBECONFIG=.kcp/consumer.kubeconfig
kubectl apply -f contrib/kcp/deploy/examples/cowboy.yaml
kubectl apply -f contrib/kcp/deploy/examples/sheriff.yaml
```




## Debug

```bash

cp .kcp/admin.kubeconfig .kcp/debug.kubeconfig
export KUBECONFIG=.kcp/debug.kubeconfig
k ws use :root:kube-bind

k -s "$(kubectl get apiexportendpointslice kube-bind.io -o jsonpath='{.status.endpoints[0].url}')/clusters/*" api-resources

k -s "$(kubectl get apiexportendpointslice kube-bind.io -o jsonpath='{.status.endpoints[0].url}')/clusters/*" get crd

namespace=$(yq '.contexts[0].context.namespace' remote.kubeconfig)

# some claimed objects

kubectl create cm provider -n "$namespace-default"
kubectl label cm provider app=wildwest -n "$namespace-default"

kubectl create cm consumer -n default
kubectl label cm consumer app=wildwest -n default

kubectl create secret generic provider-secret
kubectl label secret provider-secret app=wildwest

kubectl create namespace bob
kubectl create secret generic wildwest-secrets1 -n bob
kubectl label secret wildwest-secrets1 app=wildwest -n bob
```
