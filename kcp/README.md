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

1. Start kcp
2. Bootstrap kcp:
```bash
cp .kcp/admin.kubeconfig .kcp/provider.kubeconfig
export KUBECONFIG=.kcp/provider.kubeconfig
./bin/kcp-init --kcp-kubeconfig $KUBECONFIG
```
3. Run the backend:
```
k ws use :root:kube-bind

bin/backend \
  --multicluster-runtime-provider kcp \
  --server-url=$(kubectl get apiexportendpointslice kube-bind.io -o jsonpath="{.status.endpoints[0].url}") \
  --oidc-issuer-client-secret=ZXhhbXBsZS1hcHAtc2VjcmV0 \
  --oidc-issuer-client-id=kube-bind \
  --oidc-issuer-url=http://127.0.0.1:5556/dex \
  --oidc-callback-url=http://127.0.0.1:8080/callback \
  --pretty-name="BigCorp.com" \
  --namespace-prefix="kube-bind-" \
  --cookie-signing-key=bGMHz7SR9XcI9JdDB68VmjQErrjbrAR9JdVqjAOKHzE= \
  --cookie-encryption-key=wadqi4u+w0bqnSrVFtM38Pz2ykYVIeeadhzT34XlC1Y=
```


4. Copy the kubeconfig to the consumer:
```bash
cp .kcp/admin.kubeconfig .kcp/sub-provider.kubeconfig
export KUBECONFIG=.kcp/sub-provider.kubeconfig
k ws use :root
```

5. Run `kubectl ws create sub-provider --enter`
6. Bind the APIExport to the workspace
```bash
kubectl kcp bind apiexport root:kube-bind:kube-bind.io --accept-permission-claim clusterrolebindings.rbac.authorization.k8s.io \
  --accept-permission-claim clusterroles.rbac.authorization.k8s.io \
  --accept-permission-claim customresourcedefinitions.apiextensions.k8s.io \
  --accept-permission-claim serviceaccounts.core \
  --accept-permission-claim configmaps.core \
  --accept-permission-claim secrets.core \
  --accept-permission-claim namespaces.core \
  --accept-permission-claim serviceaccounts.rbac.authorization.k8s.io \
  --accept-permission-claim roles.rbac.authorization.k8s.io \
  --accept-permission-claim rolebindings.rbac.authorization.k8s.io
```

7. Create CRD:
```bash
kubectl apply -f deploy/examples/crd-mongodb.yaml
```

8. Get LogicalCluster:

```bash
kubectl get logicalcluster
# NAME      PHASE   URL                                                    AGE
# cluster   Ready   https://192.168.2.166:6443/clusters/2xh2v3gzjhn4tmve 
```

9. Now we gonna initiate consumer:
```bash
cp .kcp/admin.kubeconfig .kcp/consumer.kubeconfig
export KUBECONFIG=.kcp/consumer.kubeconfig
kubectl ws use :root
kubectl ws create consumer --enter
```

10. Bind the thing:

```bash
./bin/kubectl-bind http://127.0.0.1:8080/clusters/2xh2v3gzjhn4tmve/exports --dry-run -o yaml > apiserviceexport.yaml

# we need dedicated secret for that
kubectl get secret kubeconfig-wvvsb -n kube-bind -o jsonpath='{.data.kubeconfig}' | base64 -d > remote.kubeconfig

./bin/kubectl-bind apiservice --remote-kubeconfig remote.kubeconfig -f apiserviceexport.yaml  --skip-konnector --remote-namespace kube-bind-m5zx4

export KUBECONFIG=.kcp/consumer.kubeconfig
go run ./cmd/konnector/ --lease-namespace default

## Debug

```bash
cp .kcp/admin.kubeconfig .kcp/debug.kubeconfig   
export KUBECONFIG=.kcp/debug.kubeconfig
k ws use :root:kube-bind

k -s "$(kubectl get apiexportendpointslice kube-bind.io -o jsonpath="{.status.endpoints[0].url}")/clusters/*" api-resources   
k -s "$(kubectl get apiexportendpointslice kube-bind.io -o jsonpath="{.status.endpoints[0].url}")/clusters/*"  get crd