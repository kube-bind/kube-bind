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

1. Start kcp

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
3. Run the backend:
```
k ws use :root:kube-bind

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

This process will keep running, so open a new terminal.

## Provider

4. Copy the kubeconfig to the provider and create provider workspace:
```bash
cp .kcp/admin.kubeconfig .kcp/provider.kubeconfig
export KUBECONFIG=.kcp/provider.kubeconfig
k ws use :root
kubectl create-workspace provider --enter
```

5. Bind the APIExport to the provider workspace
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
  --accept-permission-claim subjectaccessreviews.authorization.k8s.io \
  --accept-permission-claim rolebindings.rbac.authorization.k8s.io \
  --accept-permission-claim apiresourceschemas.apis.kcp.io
```

6. Create CRD in provider:
```bash
kubectl apply -f contrib/kcp/deploy/examples/apiexport.yaml
kubectl apply -f contrib/kcp/deploy/examples/apiresourceschema-cowboys.yaml
kubectl apply -f contrib/kcp/deploy/examples/apiresourceschema-sheriffs.yaml
kubectl kcp bind apiexport root:provider:cowboys-stable

kubectl apply -f deploy/examples/template-cowboys.yaml
kubectl apply -f deploy/examples/template-sheriffs.yaml
kubectl apply -f deploy/examples/collection.yaml
```

7. Get LogicalCluster:

```bash
kubectl get logicalcluster
# NAME      PHASE   URL                                                    AGE
# cluster   Ready   https://192.168.2.166:6443/clusters/2ocmmccjkme8bof4
```

## Consumer

8. Now we gonna initiate consumer:
```bash
cp .kcp/admin.kubeconfig .kcp/consumer.kubeconfig
export KUBECONFIG=.kcp/consumer.kubeconfig
kubectl ws use :root
kubectl ws create consumer --enter
```

9. Bind the thing:

```bash
./bin/kubectl-bind login http://127.0.0.1:8080 --cluster 2ocmmccjkme8bof4 
./bin/kubectl-bind --dry-run -o yaml > apiserviceexport.yaml

# Extract secret for binding process. Note that secret name is not the same as output from command above. Check secret
# name by running `kubectl get secret -n kube-bind`
kubectl get secrets -n kube-bind -o jsonpath='{.items[0].data.kubeconfig}' | base64 -d > remote.kubeconfig

namespace=$(yq '.contexts[0].context.namespace' remote.kubeconfig)

./bin/kubectl-bind apiservice -v 6 --remote-kubeconfig remote.kubeconfig -f apiserviceexport.yaml --skip-konnector --remote-namespace "$namespace"
```

This will keep running, so switch to a new terminal.

### Consumer Konnector

Start konnector:

```bash
./bin/konnector --lease-namespace default --kubeconfig .kcp/consumer.kubeconfig
```

# Create an instance 

```bash
kubectl apply -f deploy/examples/cr-cowboy.yaml
kubectl apply -f deploy/examples/cr-sheriff.yaml
```