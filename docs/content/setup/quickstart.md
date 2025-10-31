---
description: >
  Get started with kube bind.
---

# Quickstart

## Prerequisites

- [kubectl](https://kubernetes.io/docs/tasks/tools/#kubectl)

## Start with kube-bind

This section allows you to run local kube-bind backend and konnector with the standard Kubernetes provider.

Starting with v0.5.0, kube-bind supports multiple backend providers through multicluster-runtime:

- **Standard Provider** (default): Works with regular Kubernetes clusters
- **kcp Provider**: Advanced multi-tenant setup with kcp workspaces (see [kcp Setup Guide](kcp-setup.md))

This quickstart uses the default provider. For kcp integration, see the [kcp Setup Guide](kcp-setup.md).

### Using kcp for Local Development

For local development, we use [kcp](https://github.com/kcp-dev/kcp) to create multiple logical clusters under a single kcp instance, making them available and accessible via the same URL.

To run kcp, you need to have a kcp binary.

```shell
$ make run-kcp
```

To run the current backend, there must be an OIDC issuer installed in place to do the
the oauth2 workflow.

We use dex to manage OIDC, following the steps below you can run a local OIDC issuer using dex:

* First, clone the dex repo: `git clone https://github.com/dexidp/dex.git`
* `cd dex` and then build the dex binary `make build`
* The binary will be created in `bin/dex`
* Adjust the config file(`examples/config-dev.yaml`) for dex by specifying the server callback method:
  ```yaml
  staticClients:
  - id: kube-bind
    redirectURIs:
    - 'http://127.0.0.1:8080/callback'
    name: 'Kube Bind'
    secret: ZXhhbXBsZS1hcHAtc2VjcmV0
  ```
* Run dex: `./bin/dex serve examples/config-dev.yaml`

Next you should be able to run the backend. For it you need a kubernetes cluster (e.g. kind)
accessible.

***Note: make sure before running the backend that you have the dex server up and running as mentioned above
and that you have at least one k8s cluster. Take a look at the backend option in the cmd/main.go file***

Create copy of kcp kubeconfig and create provider cluster:

```shell
cp .kcp/admin.kubeconfig .kcp/provider.kubeconfig
export KUBECONFIG=.kcp/provider.kubeconfig
kubectl ws create provider --enter
```

* apply the CRDs:

   ```shell
   kubectl apply -f deploy/crd
   ```
* In order to populate binding list on website, we need a CRD with label `kube-bind.io/exported: true`. Apply example APIResourceSchema for the CRD: `kubectl apply -f deploy/examples/crd-mangodb.yaml`

   ```shell
   kubectl apply -f deploy/examples/crd-mangodb.yaml
   kubectl apply -f deploy/examples/crd-foo.yaml
   kubectl apply -f deploy/examples/template-mangodb.yaml
   kubectl apply -f deploy/examples/template-foo.yaml
   kubectl apply -f deploy/examples/collection.yaml
   ```

* start the backend binary with the right flags:
   ```shell
   make build

   bin/backend \
     --oidc-issuer-client-secret=ZXhhbXBsZS1hcHAtc2VjcmV0 \
     --oidc-issuer-client-id=kube-bind \
     --oidc-issuer-url=http://127.0.0.1:5556/dex \
     --oidc-callback-url=http://127.0.0.1:8080/callback \
     --pretty-name="BigCorp.com" \
     --namespace-prefix="kube-bind-" \
     --cookie-signing-key=bGMHz7SR9XcI9JdDB68VmjQErrjbrAR9JdVqjAOKHzE= \
     --cookie-encryption-key=wadqi4u+w0bqnSrVFtM38Pz2ykYVIeeadhzT34XlC1Y= \
     --consumer-scope=cluster
   ```

where `ZXhhbXBsZS1hcHAtc2VjcmV0` matches the value of the dex config file.

The `--cookie-signing-key` and `--cookie-encryption-key` settings can be generated using:

```shell
$ openssl rand -base64 32
WQh88mNOY0Z3tLy1/WOud7qIEEBxz+POc4j8BsYenYo=
```

The `--cookie-signing-key` option is required and supports 32 and 64 byte lengths.
The `--cookie-encryption-key` option is optional and supports byte lengths of 16, 24, 32 for AES-128, AES-192, or AES-256.

### Consumer

Now create consumer cluster:

```shell
export KUBECONFIG=.kcp/admin.kubeconfig
kubectl ws create consumer --enter
```

Now create the APIServiceExportRequest:

```shell
./bin/kubectl-bind http://127.0.0.1:8080/exports --dry-run -o yaml > apiserviceexport.yaml

# This will wait for konnector to be ready. Once this gets running - start the konnector bellow
# IMPORTANT: Check namespace to be used!
./bin/kubectl-bind apiservice --remote-kubeconfig .kcp/provider.kubeconfig -f apiserviceexport.yaml  --skip-konnector --remote-namespace <namespace>

# run konnector in different terminal
export KUBECONFIG=.kcp/admin.kubeconfig
go run ./cmd/konnector/ --lease-namespace default
```

Create mangoDB object:

```bash
kubectl create -f deploy/examples/mangodb.yaml
```
