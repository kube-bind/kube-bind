<---
description: >
  Install kube-bind on an existing Kubernetes cluster via the official Helm chart.
---

# Installation with Helm

Kube-bind can be installed on an existing Kubernetes cluster using the official Helm chart.
There are 2 helm charts available: `kube-bind/backend` for service providers and `kube-bind/konnectors` for service consumers.

## Quick Start

**Important**: Current version of kube-bind uses application level redirect (HTTP 302) to CLI. Your ingress controller must support this behavior.

## Prerequisites & Setup Guides

The following prerequisites are required. Click the links below for detailed setup instructions:

- **[Kubernetes cluster](#kubernetes-cluster)** - A running Kubernetes cluster
- **[Helm 3.x](#helm)** - Package manager for Kubernetes
- **[cert-manager](#cert-manager-setup)** - For TLS certificate management
- **[OIDC provider](#oidc-provider-setup)** - For authentication (Dex, Keycloak, etc.)

### Install Kube-Bind Backend

1. **Add the Helm repository:**
```bash
helm repo add kube-bind https://kube-bind.github.io/helm-charts
helm repo update
```

2. **Configure your values:**
Edit `deploy/charts/backend/examples/values-local-development.yaml` and replace the placeholder values:
- `### REPLACE ME ###` with your actual OIDC credentials
- Update hostnames to match your setup

3. **Install the backend:**
```bash
helm upgrade --install \
    --namespace kube-bind \
    --create-namespace \
    --values ./deploy/charts/backend/examples/values-local-development.yaml \
    kube-bind kube-bind/backend
```

4. **Seed with example resources (optional):**
```bash
kubectl apply -f deploy/examples/crd-foo.yaml
kubectl apply -f deploy/examples/crd-mangodb.yaml
kubectl apply -f deploy/examples/template-foo.yaml
kubectl apply -f deploy/examples/template-mangodb.yaml
kubectl apply -f deploy/examples/collection.yaml
```

That's it! Your kube-bind backend is now ready to use.

---

### Kubernetes Cluster
You need a running Kubernetes cluster with `kubectl` configured. For testing, you can create a local cluster:

```bash
kind create cluster --name kube-bind-test
```

### Helm
Install Helm 3.x from [https://helm.sh/docs/intro/install/](https://helm.sh/docs/intro/install/)

### cert-manager Setup

Install cert-manager for automatic TLS certificate management:

```bash
helm repo add jetstack https://charts.jetstack.io
helm repo update

helm upgrade \
  --install \
  --namespace cert-manager \
  --create-namespace \
  --version v1.18.2 \
  --set crds.enabled=true \
  --atomic \
  cert-manager jetstack/cert-manager
```

#### Configure Let's Encrypt (Optional)

Create a ClusterIssuer for automatic certificate provisioning:

```bash
kubectl apply -f - <<EOF
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: letsencrypt-prod
spec:
  acme:
    # Replace with your email address
    email: admin@example.com
    server: https://acme-v02.api.letsencrypt.org/directory
    privateKeySecretRef:
      name: le-issuer-account-key
    solvers:
    - dns01:
        cloudflare:
          email: admin@example.com
          apiKeySecretRef:
            name: cloudflare-api-key-secret
            key: api-key
EOF
```

Create the Cloudflare API key secret:

```bash
kubectl apply -f - <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: cloudflare-api-key-secret
  namespace: cert-manager
type: Opaque
data:
  # Replace with your base64 encoded Cloudflare Global API Key
  # Get your API key from: https://dash.cloudflare.com/profile/api-tokens
  # Then encode it: echo -n "your-api-key" | base64
  api-key: #### REPLACE ME WITH BASE64 ENCODED VALUE ####
EOF
```

### OIDC Provider Setup

Kube-bind requires an OIDC provider for authentication. Here's how to set up Dex as an example:

#### Install Dex OIDC Provider

```bash
kubectl create namespace oidc

# Request TLS certificate for Dex
kubectl apply -f - <<EOF
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: dex-tls-cert
  namespace: oidc
spec:
  secretName: dex-tls
  issuerRef:
    name: letsencrypt-prod
    kind: ClusterIssuer
    group: cert-manager.io
  dnsNames:
  - auth.example.com  # Replace with your domain
  usages:
  - digital signature
  - key encipherment
EOF
```

Wait for the certificate to be issued, then install Dex:

```bash
helm repo add dex https://charts.dexidp.io

cat > /tmp/dex-values.yaml <<EOF
config:
  issuer: https://auth.example.com  # Replace with your domain
  
  logger:
    level: "debug"
  
  storage:
    type: kubernetes
    config:
      inCluster: true
  
  web:
    https: 0.0.0.0:5557
    tlsCert: /etc/dex/tls/tls.crt
    tlsKey: /etc/dex/tls/tls.key
    http: "0.0.0.0:5556"
  
  connectors:
    - type: github
      id: github
      name: GitHub
      config:
        clientID: ### REPLACE ME ###
        clientSecret: ### REPLACE ME ###
        redirectURI: https://auth.example.com/callback
        org: your-org  # Replace with your GitHub org
  
  staticClients:
    - id: kube-bind
      redirectURIs:
      - https://auth.example.com/callback
      - http://localhost:8000
      - https://kube-bind.example.com/callback  # Replace with your domain
      name: 'KubeBindApp'
      secret: ### REPLACE ME ###

service:
  type: LoadBalancer
  ports:
    https:
      port: 443
    http:
      port: 5556

https:
  enabled: true

volumes:
- name: tls-cert
  secret:
    secretName: dex-tls

volumeMounts:
- name: tls-cert
  mountPath: /etc/dex/tls
  readOnly: true
EOF

helm upgrade -i dex dex/dex \
  --create-namespace \
  --namespace oidc \
  -f /tmp/dex-values.yaml
```

---

## Configuration

The example values file at `deploy/charts/backend/examples/values-local-development.yaml` contains all the configuration options. Make sure to replace all `### REPLACE ME ###` placeholders with your actual values:

- **OIDC credentials**: Get these from your OIDC provider (Dex, GitHub, etc.)
- **Cookie keys**: Generate with `openssl rand -base64 32`
- **Hostnames**: Update to match your actual domains

For production deployments, create your own values file based on the example.