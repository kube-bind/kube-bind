# Local kube-bind with kind

This guide will take you through the steps necessary to set up `kube-bind` locally for testing and development.

After following it you'll end up with:
- Two local `kind` clusters: a **provider** and a **consumer**
- The `example-backend` running on the host
- A mangodb example CRD bound into the consumer cluster

## Prerequisites

### Tools

These need to be installed on the host and will be used in the steps below

1. [`kind`](https://kind.sigs.k8s.io/docs/user/quick-start/#installation): to create local clusters
2. [`kubectl`](https://kubernetes.io/docs/tasks/tools/): to interact with those clusters
3. [`kubectx`](https://github.com/ahmetb/kubectx): to switch between cluster configs (not required, but convenient)
4. A working go compiler

### Oauth2 identity provider setup

This is needed later on for `kubectl bind` to authenticate against the example backend, when binding the example provider to the consumer cluster.

We're going to use `dex`, just like the kube-bind README suggests.

Follow these steps to set it up:
1. Clone dex repository:
```
git clone https://github.com/dexidp/dex.git -b v2.37.0
cd dex/
```
2. Build with
```
make
```
3. Create `dex-config.yaml`
```
staticClients:
  - id: kube-bind
    redirectURIs:
      - 'http://127.0.0.1:8080/callback' # << points to example-backend
    name: 'Kube Bind'
    secret: ZXhhbXBsZS1hcHAtc2VjcmV0

issuer: http://127.0.0.1:5556/dex
storage:
  type: sqlite3
  config:
    file: dex.db
web:
  http: 127.0.0.1:5556
telemetry:
  http: 127.0.0.1:5558
grpc:
  addr: 127.0.0.1:5557
connectors:
  - type: mockCallback
    id: mock
    name: Example
enablePasswordDB: true
staticPasswords:
  - email: "admin@example.com"
    hash: "$2a$10$2b2cU8CPhOTaGrs1HRQuAueS7JTT5ZHsHSzYiFPm1leZck7Mc8T4W"
    username: "admin"
    userID: "08a8684b-db88-4b73-90a9-3cd1661f5466"
```
4. Start dex:
```
bin/dex serve dex-config.yaml
```

Keep this running in the background and move on to the actual **Setup** below.

## Setup

1. Clone kube-bind repository:
```
git clone https://github.com/kube-bind/kube-bind.git
cd kube-bind/
```
2. Compile:
```
make build
```
3. Create two config files, for the kind clusters:
- `provider.yaml`:
```
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
name: provider
networking:
  apiServerAddress: 192.168.178.80 # << REPLACE
```
- `consumer.yaml`:
```
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
name: consumer
networking:
  apiServerAddress: 192.168.178.80 # << REPLACE
```

The `apiServerAddress` must be replaced with the **local IP address of the host** (your computer).
Using 127.0.0.1 will not work, since this address must be reachable from the nodes of the respective other cluster which have their own loopback interfaces.

4. Create both clusters:
```
kind create cluster --config provider.yaml
kind create cluster --config consumer.yaml
```
5. Switch to provider cluster:
```
kubectx kind-provider
```
6. Create various CRDs:
```
# kube-bind's own CRDs
kubectl apply -f deploy/crd
# An example resource ("MangoDB") for the example-backend
kubectl apply -f test/e2e/bind/fixtures/provider
```
7. Start example backend:
```
bin/example-backend \
  --oidc-issuer-client-secret=ZXhhbXBsZS1hcHAtc2VjcmV0 \
  --oidc-issuer-client-id=kube-bind \
  --oidc-issuer-url=http://127.0.0.1:5556/dex \
  --oidc-callback-url=http://127.0.0.1:8080/callback \
  --pretty-name="BigCorp.com" \
  --namespace-prefix="kube-bind-" \
  --cookie-signing-key=bGMHz7SR9XcI9JdDB68VmjQErrjbrAR9JdVqjAOKHzE= \
  --cookie-encryption-key=wadqi4u+w0bqnSrVFtM38Pz2ykYVIeeadhzT34XlC1Y=
  ```

Leave this running, and continue with the **Usage** section

## Usage

1. `cd` to the `kube-bind` source tree, we'll need to use `kubectl-bind` from there:
```
cd path/to/kube-bind
```
(you can also install it globally and invoke it as `kubectl bind` - but that is out of scope for this guide)

2. Target the consumer cluster:
```
kubectx kind-consumer
```
3. Bind to the example service:
```
./bin/kubectl-bind http://localhost:8080/export
```
This should prompt you to open a URL in your browser.
Click:
- "Log in with Example"
- "Grant Access"
- Click "Bind" on the "mangodb" resource
4. Back in the terminal, you should see something like this:
```
🔑 Successfully authenticated to http://localhost:8080/export
🔒 Created secret kube-bind/kubeconfig-whs9m for host https://192.168.178.80:57303, namespace kube-bind-2xdr6
🚀 Executing: kubectl bind apiservice --remote-kubeconfig-namespace kube-bind --remote-kubeconfig-name kubeconfig-whs9m -f -
✨ Use "-o yaml" and "--dry-run" to get the APIServiceExportRequest and pass it to "kubectl bind apiservice" directly. Great for automation.
🚀 Deploying konnector v0.3.0 to namespace kube-bind.
  Waiting for the konnector to be ready.....
✅ Created APIServiceBinding mangodbs.mangodb.com

NAME                                                  PROVIDER   READY   MESSAGE   AGE
apiservicebinding.kube-bind.io/mangodbs.mangodb.com              True              0s
```
4. To verify that everything worked, check if there is a new "mangodb" CRD in the consumer cluster:
```
kubectl get crds
NAME                              CREATED AT
apiservicebindings.kube-bind.io   2023-11-14T14:11:58Z
mangodbs.mangodb.com              2023-11-14T14:12:01Z
```
5. Create a mangodb CR, by applying this manifest:
```
apiVersion: mangodb.com/v1alpha1
kind: MangoDB
metadata:
  name: my-db
spec:
  tokenSecret: not-sure-if-this-is-used
```
6. Switch to the provider cluster, and verify that the resource was mirrored there:
```
kubectx kind-provider
kubectl get mangodbs -A
```
You should see one object, named `my-db` in a namespace starting with `kube-bind-`.

## Cleanup

To clean up, simply stop the `example-backend` and `dex` process, then remove the two clusters:
```
kind delete cluster --name provider
kind delete cluster --name consumer
```
