# Local kube-bind with kind

This guide will walk you through setting up kube-bind between two Kubernetes clusters, where
* **Backend cluster**:
  * Deploys: dex, cert-manager, kube-bind/example-backend
  * Provides: kube-bind compatible backend for MangoDB resources
* **App cluster**:
  * Provides: an application consuming MangoDBs

## Pre-requisites

To start, you'll need following tools available in your system or a VM:

* [`kind`](https://kind.sigs.k8s.io/docs/user/quick-start/#installation)
* [`kubectl`](https://kubernetes.io/docs/tasks/tools/)
* [`kubectl-bind`](https://github.com/kube-bind/kube-bind/releases/latest) (a kubectl plugin)
* [`helm`](https://helm.sh/docs/intro/quickstart/)
* [`jq`](https://jqlang.github.io/jq/download/)

To install `kubectl-bind` plugin, please download the archive for your platform from the link above, extract it, and place the `kubectl-bind` executable in your system's `$PATH`.

> Tip: In case of encountering `Too many open files` error when deploying the Kind clusters, run following commands:
>
> ```sh
> sudo sysctl fs.inotify.max_user_watches=524288
> sudo sysctl fs.inotify.max_user_instances=512
> ```
>
> See https://kind.sigs.k8s.io/docs/user/known-issues/#pod-errors-due-to-too-many-open-files for more details.

## Backend cluster

The backend cluster we'll prepare in this section will provide a kube-bind compatible backend that will provide a controller for a demo resource "MangoDB" we'll consume in another cluster later.

> What is MangoDB? It is just an example CRD to demonstrate kube-bind's capabilities and testing, without any workloads. See its definition in [/test/e2e/bind/fixtures/provider/crd-mangodb.yaml](/test/e2e/bind/fixtures/provider/crd-mangodb.yaml).

### Step one: create the Backend cluster

First, stash the host's external IP in a variable as we're going to use it often:

```sh
export BACKEND_HOST_IP="$(hostname -i | cut -d' ' -f1)"
```

Create a Kind cluster named "backend":

```sh
cat << EOF_BackendClusterDefinition | kind create cluster --config=-
apiVersion: kind.x-k8s.io/v1alpha4
kind: Cluster
name: backend
networking:
  apiServerAddress: ${BACKEND_HOST_IP}
nodes:
- role: control-plane
  extraPortMappings:
  # MangoDB export endpoint
  - containerPort: 30080
    hostPort: 8080
    protocol: TCP
  # DEX endpoint
  - containerPort: 30556
    hostPort: 5556
    protocol: TCP
EOF_BackendClusterDefinition
```

> Note: the port mappings will become clear later on, but in general this setup is solely specific to how Kind exposes ports of its nodes on the host. Specifically, we're exposing ports from containers through NodePort services on Kind's nodes, and to make these ports available on the host we need to map them to host's ports through `extraPortMappings`.

### Step two: deploy an identity provider

kube-bind relies on OAuth2 for securely authenticating consumer and producer clusters. There are many ways to handle that in Kubernetes, for example with [DEX IDP](https://github.com/dexidp/dex). It depends on cert-manager, which we'll deploy first:

```sh
helm repo add jetstack https://charts.jetstack.io
helm install \
    --create-namespace \
    --namespace pki \
    --version v1.16.2 \
    --set crds.enabled=true \
    cert-manager jetstack/cert-manager
```

And now let's deploy DEX:

```sh
helm repo add dex https://charts.dexidp.io
cat << EOF_DEXDeploymentConfig |
config:
    staticClients:
      - id: kube-bind
        redirectURIs:
          - 'http://${BACKEND_HOST_IP}:8080/callback'
        name: 'Kube Bind'
        secret: ZXhhbXBsZS1hcHAtc2VjcmV0

    issuer: http://${BACKEND_HOST_IP}:5556/dex

    storage:
      type: kubernetes
      config:
        inCluster: true

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
        hash: "\$2a\$10\$2b2cU8CPhOTaGrs1HRQuAueS7JTT5ZHsHSzYiFPm1leZck7Mc8T4W"
        username: "admin"
        userID: "08a8684b-db88-4b73-90a9-3cd1661f5466"
EOF_DEXDeploymentConfig
helm install \
    --create-namespace \
    --namespace idp \
    --set service.type=NodePort \
    --set service.ports.http.nodePort=30556 \
    dex dex/dex \
    -f -
```

### Step three: deploy the MangoDB kube-bind backend

Now we'll deploy a kube-bind--compatible backend for MangoDB. Let's start with kube-bind CRDs:

```sh
kubectl apply -f deploy/crd
```

And now CRDs for MangoDB:

```sh
kubectl apply -f test/e2e/bind/fixtures/provider/crd-mangodb.yaml
```

To set up the MangoDB backend we'll need:
* ServiceAccount and ClusterRoleBinding for kube-bind's user,
* Deployment that runs the MangoDB backend
* Service that exposes the backend's address

```sh
kubectl create namespace backend
# This is the address that will be used when generating kubeconfigs the App cluster,
# and so we need to be able to reach it from outside.
export BACKEND_KUBE_API_EXTERNAL_ADDRESS="$(kubectl config view --minify -o json | jq '.clusters[0].cluster.server' -r)"
# For demo example let's just bind "cluster-admin" ClusterRole to backend's "default" ServiceAccount.
kubectl create clusterrolebinding backend-admin --clusterrole cluster-admin --serviceaccount backend:default
# Create a new Deployment for the MangoDB backend.
kubectl --namespace backend \
    create deployment mangodb \
    --image ghcr.io/kube-bind/example-backend:v0.4.6 \
    --port 8080 \
    -- /ko-app/example-backend \
        --listen-address 0.0.0.0:8080 \
        --external-address "${BACKEND_KUBE_API_EXTERNAL_ADDRESS}" \
        --oidc-issuer-client-secret=ZXhhbXBsZS1hcHAtc2VjcmV0 \
        --oidc-issuer-client-id=kube-bind \
        --oidc-issuer-url=http://${BACKEND_HOST_IP}:5556/dex \
        --oidc-callback-url=http://${BACKEND_HOST_IP}:8080/callback \
        --pretty-name="BigCorp.com" \
        --namespace-prefix="kube-bind-" \
        --cookie-signing-key=bGMHz7SR9XcI9JdDB68VmjQErrjbrAR9JdVqjAOKHzE= \
        --cookie-encryption-key=wadqi4u+w0bqnSrVFtM38Pz2ykYVIeeadhzT34XlC1Y=
# Expose mangodb's container port 8080 as a NodePort at 30080. We've already configured
# Kind to expose 30800 at host's 8080.
kubectl --namespace backend \
    create service nodeport mangodb \
    --tcp 8080 \
    --node-port 30080
```

And that's really all there's to it. After that, you should see a kubectl output similar to this:

```shell
$ kubectl --namespace backend get all
NAME                          READY   STATUS    RESTARTS   AGE
pod/mangodb-6ff44cbbf-x7cjm   1/1     Running   0          100s

NAME              TYPE       CLUSTER-IP     EXTERNAL-IP   PORT(S)          AGE
service/mangodb   NodePort   10.96.10.212   <none>        8080:30080/TCP   100s

NAME                      READY   UP-TO-DATE   AVAILABLE   AGE
deployment.apps/mangodb   1/1     1            1           100s

NAME                                DESIRED   CURRENT   READY   AGE
replicaset.apps/mangodb-6ff44cbbf   1         1         1       100s
```

## App cluster

The App cluster will consume MangoDB CRs provided by the Backend.

### Step one: create the Backend cluster

Again, let's start by stashing the host's external IP in a variable as we're going to use it often (possibly the same one as for the Backend cluster):

```sh
export APP_HOST_IP="$(hostname -i | cut -d' ' -f1)"
```

Create a Kind cluster named "app":

```sh
cat << EOF_AppClusterDefinition | kind create cluster --config=-
apiVersion: kind.x-k8s.io/v1alpha4
kind: Cluster
name: app
networking:
  apiServerAddress: ${APP_HOST_IP}
EOF_AppClusterDefinition
```

### Binding MangoDB backend

Now we'll bring in MangoDB CRDs from the Backend cluster (you can run `kubectl get crds` to see there are none yet):

```sh
$ kubectl bind http://${BACKEND_HOST_IP}:8080/export
DISCLAIMER: This is a prototype. It will change in incompatible ways at any time.

📦 Created kube-bind namespace.



To authenticate, visit in your browser:

	http://${BACKEND_HOST_IP}:8080/authorize?c=3QnoGw&p=39595&s=b2YLH6
```

The client is now waiting for you to visit the address similar to the one displayed in the output above. After completing the steps to create an OAuth2 token, it is then used by the kube-bind backend to pass the ServiceAccount's kubeconfig (in the Backend cluster) to the App cluster securely:
1. on the "Log in to dex" landing page, select "Log in with Example",
2. on the "Grant Access" page, click the "Grant Access" button,
3. lastly, click "Bind" when the page displays the mangodb resource.

Go back to the terminal where `kubectl bind` command was run, and you should see the following output:
```
🔑 Successfully authenticated to http://${BACKEND_HOST_IP}:8080/export
🔒 Created secret kube-bind/kubeconfig-x9bd5 for host https://${BACKEND_HOST_IP}:34595, namespace kube-bind-gfsqn
🚀 Executing: kubectl bind apiservice --remote-kubeconfig-namespace kube-bind --remote-kubeconfig-name kubeconfig-x9bd5 -f -
✨ Use "-o yaml" and "--dry-run" to get the APIServiceExportRequest.
   and pass it to "kubectl bind apiservice" directly. Great for automation.
🚀 Deploying konnector v0.4.6 to namespace kube-bind.
   Waiting for the konnector to be ready..............
✅ Created APIServiceBinding mangodbs.mangodb.com

NAME                                                  PROVIDER   READY   MESSAGE   AGE
apiservicebinding.kube-bind.io/mangodbs.mangodb.com              False   Pending   0s
```

### Step two: demo time!

Let's see if we have CRDs for the MangoDB resource:

```sh
$ kubectl get crds
NAME                              CREATED AT
apiservicebindings.kube-bind.io   2024-12-19T08:46:13Z
mangodbs.mangodb.com              2024-12-19T08:46:17Z
```

We do! Now create a CR for it:

```sh
kubectl create -f - << EOF_MangoDBDefinition
apiVersion: mangodb.com/v1alpha1
kind: MangoDB
metadata:
  name: my-db
spec:
  tokenSecret: my-secret
  region: eu-west-1
  tier: Shared
EOF_MangoDBDefinition
kubectl describe mangodb my-db
```

And finally, switch to the backend cluster and see that the CR is mirrored there:
```sh
$ kubectl config use-context kind-backend
Switched to context "kind-backend".
# Your "kube-bind-<Generated string>-default" will be different.
$ kubectl -n kube-bind-rp2s9-default describe mangodb my-db
Name:         my-db
Namespace:    kube-bind-rp2s9-default
Labels:       <none>
Annotations:  <none>
API Version:  mangodb.com/v1alpha1
Kind:         MangoDB
Metadata:
  Creation Timestamp:  2024-12-19T08:48:07Z
  Generation:          1
  Resource Version:    1564
  UID:                 bed9f6d6-79d5-4535-8b20-690470b23378
Spec:
  Backup:        false
  Region:        eu-west-1
  Tier:          Shared
  Token Secret:  my-secret
Events:          <none>
```

### Step three: clean up

Once you're done, you may clean up the setup simply by deleting the two kind clusters:

```sh
kind delete cluster --name backend
kind delete cluster --name app
```
