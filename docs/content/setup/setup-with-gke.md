# Deplopying Kube-Bind into GKE clusters

This guide will walk you through setting up kube-bind between two Kubernetes clusters running in GKE, where

**Backend cluster**:
  * Deploys dex, cert-manager and kube-bind/example-backend
  * Provides kube-bind compatible backend for MangoDB resources

**App cluster**:
  * Provides an application consuming MangoDBs

## Pre-requisites

To start, you'll need following tools available in your system or a VM:

* [`kind`](https://kind.sigs.k8s.io/docs/user/quick-start/#installation)
* [`kubectl`](https://kubernetes.io/docs/tasks/tools/)
* [`kubectl-bind`](https://github.com/kube-bind/kube-bind/releases/latest) (a kubectl plugin)
* [`helm`](https://helm.sh/docs/intro/quickstart/)
* [`jq`](https://jqlang.github.io/jq/download/)

To install `kubectl-bind` plugin, please download the archive for your platform from the link above, extract it, and place the `kubectl-bind` executable in your system's `$PATH`.

## Provider cluster

The provider cluster we'll prepare in this section will provide a kube-bind compatible backend that will provide a controller for a demo resource "MangoDB" we'll consume in another cluster later.

> What is MangoDB? It is just an example CRD to demonstrate kube-bind's capabilities and testing, without any workloads. See its definition in [/test/e2e/bind/fixtures/provider/crd-mangodb.yaml](/test/e2e/bind/fixtures/provider/crd-mangodb.yaml).

### Step zero: Images

Get images either from our Github Container registry: https://github.com/orgs/kube-bind/packages?repo_name=kube-bind 
or build them locally: `make build image-local` and publish to your own registry.

### Step one: create the Backend cluster

Get yourself a vanilla GKE cluster:

```
gcloud container clusters get-credentials kube-bind-provider --region us-central1 --project kube-bind
export KUBECONFIG=provider.kubeconfig
```


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


Homepage URL: The public URL where Dex will be hosted, e.g., https://dex.dev.genericcontrolplane.io
Authorization callback URL: This is the most critical part. It must be your Dex public URL followed by /callback. For example: https://dex.dev.genericcontrolplane.io/callback.

For this write-up we use demo secrets.

```sh
cat << EOF_ClusterIssuer |
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: letsencrypt-staging
spec:
  acme:
    # The ACME server URL for Let's Encrypt's staging environment.
    server: https://acme-staging-v02.api.letsencrypt.org/directory
    # Email address used for ACME registration.
    email: mangirdas@judeikis.lt
    # Name of a secret used to store the ACME account private key.
    privateKeySecretRef:
      name: letsencrypt-staging-account-key
    # Enable the HTTP-01 challenge provider.
    solvers:
    - http01:
        ingress:
          class: gce # This is the default Ingress class on GKE
EOF_ClusterIssuer
kubectl apply -f -
```

```sh
helm repo add dex https://charts.dexidp.io
cat << EOF_DEXDeploymentConfig |
config:
  # The issuer URL is still the public HTTPS URL.
  issuer: https://dex.dev.genericcontrolplane.io

  storage:
    type: kubernetes
    config:
      inCluster: true

  # CORRECTED: Dex itself should now listen on plain HTTP inside the cluster.
  # TLS termination will be handled by the Ingress.
  web:
    http: 0.0.0.0:5556

  # Your connectors and staticClients remain the same...
  connectors:
    - type: mockCallback
      id: mock
      name: Example
  staticClients:
    - id: kube-bind
      redirectURIs:
        - 'https://dex.dev.genericcontrolplane.io/callback'
      name: 'Kube Bind'
      secret: ZXhhbXBsZS1hcHAtc2VjcmV0

# This section configures the Kubernetes Service for Dex.
# It should be ClusterIP because the Ingress will route traffic to it.
service:
  type: ClusterIP

# This section re-enables and configures the Ingress for cert-manager
ingress:
  enabled: true
  # Use 'gce' for the default GKE Ingress controller.
  # For newer GKE versions, you might not need the class annotation at all.
  className: "gce"
  annotations:
    # Use the staging issuer first for testing.
    cert-manager.io/cluster-issuer: "letsencrypt-staging"
    # This annotation is for GKE to use a static IP.
    kubernetes.io/ingress.global-static-ip-name: "dex-static-ip"
  hosts:
    - host: dex.dev.genericcontrolplane.io
      paths:
        - path: /
          pathType: ImplementationSpecific
  tls:
   # This tells the Ingress to use a certificate for the specified host.
   # Cert-manager will see this, create a Certificate resource,
   # and automatically populate the 'dex-tls' secret with the new cert.
   - secretName: dex-tls
     hosts:
       - dex.dev.genericcontrolplane.io
EOF_DEXDeploymentConfig
helm upgrade -i \
    --create-namespace \
    --namespace dex \
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
    --image ghcr.io/kube-bind/example-backend:v0.5.0-rc1 \
    --port 8080 \
    -- /ko-app/example-backend \
        --listen-address 0.0.0.0:8080 \
        --external-address "${BACKEND_KUBE_API_EXTERNAL_ADDRESS}" \
        --oidc-issuer-client-secret=xxxxxxxxxxxx== \
        --oidc-issuer-client-id=faros \
        --oidc-issuer-url=https://auth.faros.sh \
        --oidc-callback-url=http://xxxxxxxxxx:8080/callback \
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

```sh
kubectl --namespace backend expose deployment mangodb --type=LoadBalancer --port=8080 --target-port=8080
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

### Step one: create the App cluster

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
  name: bob-the-database
spec:
  tokenSecret: my-secret
  region: eu-west-1
  tier: Shared
EOF_MangoDBDefinition
kubectl describe mangodb bob-the-database
```


kubectl bind http://api.faros.sh:8080/export --konnector-image ghcr.io/kube-bind/konnector:v0.5.0-rc1   

In the provider now:

```sh
kubectl patch mangodb bob-the-database -n kube-bind-znxkg-default --subresource status --type='merge' -p='{"status":{"phase":"Running"}}'
```


kind creaet cluster --name jon 