# Kube-Bind for KCP

This is example backend for KCP that uses [kube-bind](https://github.com/kube-bind/kube-bind) to bind api-exports.

Values here should match the values used to start kcp with so that the oidc tokens are valid.
We use kcp from [kcp-dev/kcp/contrib/kcp-dex](https://github.com/kcp-dev/kcp/tree/main/contrib/kcp-dex) as an example.


## Quickstart

1. Start kcp instance with dex IDP provider:

Follow [README](https://github.com/kcp-dev/kcp/blob/main/contrib/kcp-dex/README.md) in kcp repository to have kcp with IDP running.
Just use `docs/dex-config.yaml` instead of one in kcp repistory. kube-bind version contains required callback urls for kube-bind to work.

Once this is done you should have `dex` running with custom configuration in one terminal, and kcp using this dex as IDP in another.

2. Start kube-bind backend.

```bash
make build

# bootstrap kcp instance with required workspaces and exports:
# `make run-dev-init` is make target for command below.
export KCP_REPO_DIR=${GOPATH}/src/github.com/kcp-dev/kcp/
export KCP_KUBECONFIG=${KCP_REPO_DIR}/.kcp/admin.kubeconfig
bin/bootstrap init --kcp-kubeconfig=$KCP_KUBECONFIG

# once it boostrap, start kube-bind backend. Make sure `oidc-issuer-client-secret` matches one, used in dex.
# `make run-dev` is make target for command below.

bin/backend start \
	-v 4 \
	--tls-cert-file=${KCP_REPO_DIR}127.0.0.1.pem \
	--tls-key-file=../../127.0.0.1.pem \
	--listen-address=127.0.0.1:6443 \
	--oidc-issuer-client-secret=Z2Fyc2lha2FsYmlzdmFuZGVuekWplCg== \
	--oidc-issuer-client-id=kcp-dev \
	--oidc-issuer-url=https://127.0.0.1:5556/dex \
	--oidc-callback-url=https://127.0.0.1:6443/callback \
	--oidc-authorize-url=https://127.0.0.1:6443/authorize \
	--oidc-ca-file=../../127.0.0.1.pem \
	--pretty-name="CorpAAA.com" \
	--namespace-prefix="kube-bind-" \
	--cookie-signing-key=bGMHz7SR9XcI9JdDB68VmjQErrjbrAR9JdVqjAOKHzE= \
	--cookie-encryption-key=wadqi4u+w0bqnSrVFtM38Pz2ykYVIeeadhzT34XlC1Y= \
	--workspace-path="root:kube-bind" \
	--apiexport-name="kube-bind.io" \
	--kubeconfig=${KCP_KUBECONFIG} \
	--dev-mode=true
```

3. Try example `mangodb` as consumer.

Exec init `kube-bind` backend:

```
docker exec -it docker-compose-kube-bind-1 sh

kubectl ws create mangodb --enter
kubectl kcp bind apiexport root:kube-bind:kube-bind.io --name kube-bind.io
```

Create crd for mangodb:
```
kubectl create -f https://raw.githubusercontent.com/kube-bind/kube-bind/refs/heads/main/deploy/examples/crd-mangodb.yaml
```

At this point you will need to restart kube-bind backend to get it running.
TODO(mjudeikis): Fix this.

From outside create separete kind cluster to be consumer

```
kubectl bind --insecure-skip-tls-verify  https://0.0.0.0:6444/export
```







# Raodmap & limitations

Current implementation works same way as existing `example-backend`. It allows export `crds` from worksapce to 
external Kubernetes clusters. In a way this example now makes `kube-bind` to be usable as kcp enabled service.
This should change with roadmap being implemented.

1. Add e2e tests for kcp backend to test basic behaviour.
2. Extend `kcp` and `kube-bind` to be able to export `APIBinding` resources. After this is done, you should be able to 
export native `kcp` bindings via `kube-bind`. This will allow more native `kcp` to `k8s` integration.
3. Extend `kcp` APIBindings/PermissionsClaims to be more extendable for `kube-bind` use.
4. Implement kcp native `konnector` agent. After this is done, you will be able to do `kcp` to `kcp` bindings. Currently this is 
not possible due to need to run connector agents on the consumer end.