# kcp init

kcp requires initial setup to be run before it can be used. 
This included setting up workspace/provider and setting up all the APIResourceSchemas and APIExports.

It was its own go module to avoid kcp dependencies in the main kube-bind module.

This is not required if you doing deeper integration, and controlling the setup with your own scripts.

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
  --server=$(kubectl get apiexportendpointslice kube-bind.io -o jsonpath="{.status.endpoints[0].url}") \
  --oidc-issuer-client-secret=ZXhhbXBsZS1hcHAtc2VjcmV0 \
  --oidc-issuer-client-id=kube-bind \
  --oidc-issuer-url=http://127.0.0.1:5556/dex \
  --oidc-callback-url=http://127.0.0.1:8080/callback \
  --pretty-name="BigCorp.com" \
  --namespace-prefix="kube-bind-" \
  --cookie-signing-key=bGMHz7SR9XcI9JdDB68VmjQErrjbrAR9JdVqjAOKHzE= \
  --cookie-encryption-key=wadqi4u+w0bqnSrVFtM38Pz2ykYVIeeadhzT34XlC1Y=
```


6. Run `kubectl ws create sub-provider --enter`
7. Bind the APIExport to the workspace
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
```

8. Create CRD:
```bash
kubectl apply -f deploy/examples/crd-mangodb.yaml
```


9. No we gonna imitate consumer:

```bash
cp .kcp/admin.kubeconfig .kcp/consumer.kubeconfig
export KUBECONFIG=.kcp/consumer.kubeconfig
kubectl ws use :root
kubectl ws create consumer --enter
```





## Debug

```bash
cp .kcp/admin.kubeconfig .kcp/debug.kubeconfig   
export KUBECONFIG=.kcp/debug.kubeconfig
k ws use :root:kube-bind

k -s "$(kubectl get apiexportendpointslice kube-bind.io -o jsonpath="{.status.endpoints[0].url}")/clusters/*"     api-resources   
k -s "$(kubectl get apiexportendpointslice kube-bind.io -o jsonpath="{.status.endpoints[0].url}")/clusters/*"  get crd