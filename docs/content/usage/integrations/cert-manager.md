---
title: Cert-Manager
description: |
    Guide on integrating kube-bind with cert-manager for automated TLS certificate management.
weight: 10
---

# Cert-Manager Integration

1. **Install cert-manager** in your Kubernetes cluster, where kube-bind backend is running, if you haven't already. You can follow the official installation guide [here](https://cert-manager.io/docs/installation/kubernetes/).


2. **Create a `kube-bind` template for `Certificate` resources** to allow service consumers to request TLS certificates. Below is an example template:

```yaml
kubectl apply -f - <<EOF
apiVersion: kube-bind.io/v1alpha2
kind: APIServiceExportTemplate
metadata:
  labels:
    provider: cert-manager
  name: certificate
spec:
  permissionClaims:
  - group: ""
    resource: secrets
    selector:
      references:
        - resource: certificates
          group: cert-manager.io
          jsonPath:
            name: 'spec.secretName'
  resources:
  - group: cert-manager.io
    resource: certificates
    versions:
    - v1
  scope: Namespaced
EOF
```

3. **Login into the kube-bind CLI** and request a binding to the `certificate` template created above. This will allow you to create `Certificate` resources in your consumer cluster.

```bash
kubectl bind login https://kube-bind.example.com
# you will get redirected to UI to authenticate and pick the template
kubectl bind
```

4. **Wait for the binding to be established.** Once the binding is active, you can create `Certificate` resources in your consumer cluster, and you will get `Certificate` objects synced from the provider cluster.

```bash
kubectl bind
🌐 Opening kube-bind UI in your browser...
    https://kube-bind.genericcontrolplane.io?redirect_url=....

Browser opened successfully
Waiting for binding completion from UI...
   (Press Ctrl+C to cancel)

Binding completed successfully!
Created kube-bind namespace.
🔒 Created secret kube-bind/kubeconfig-p6mfh for host https://api.kcp-prod.kcp.internal.canary.k8s.ondemand.com:443, namespace kube-bind-dkxkx
🚀 Deploying konnector v0.6.0 to namespace kube-bind with custom image "ghcr.io/kube-bind/konnector:v0.6.0-rc1".
   Waiting for the ...................
✅ Created APIServiceBinding certificate for 1 resources
Created 1 APIServiceBinding(s):
  - certificate
Resources bound successfully!
```

5. **Create a `Certificate` resource** in your consumer cluster. The cert-manager in the provider cluster will handle the issuance and management of the TLS certificate.

!!! note
        my-selfsigned-issuer must be present in the provider cluster for this example to work.

```yaml
kubectl apply -f - <<EOF
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: my-tls-cert
  namespace: default
spec:
  commonName: my-ca
  isCA: true
  issuerRef:
    kind: ClusterIssuer
    name: my-selfsigned-issuer
  secretName: my-tls-cert
EOF
```

6. Observe that the `Certificate` resource is created in the consumer cluster and the corresponding TLS secret is generated.

```bash
kubectl get certificates
NAME          READY   SECRET        AGE
my-tls-cert   True    my-tls-cert   6m55s
kubectl get secrets
NAME          TYPE                DATA   AGE
my-tls-cert   kubernetes.io/tls   3      6m33s
```
