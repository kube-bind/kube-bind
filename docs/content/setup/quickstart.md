---
description: >
  Get started with kube bind.
---

# Quickstart

## Prerequisites

- [kubectl](https://kubernetes.io/docs/tasks/tools/#kubectl)
- [kube-bind CLI](kubectl-plugin.md) installed

## Start with kube-bind

### Quick Development Setup

**Note:** This setup requires Docker as it uses advanced network configuration to minimize external DNS dependencies and enable inter-cluster connectivity.

For a quick development environment with local Kind clusters, use the `kubectl bind dev` command:

```bash
kubectl bind dev create
```

This command will:
- Create two Kind clusters: `kind-provider` (API provider) and `kind-consumer` (API consumer)
- Deploy the kube-bind backend to the provider cluster
- Generate kubeconfig files for both clusters
- Set up local DNS entries for `kube-bind.dev.local`
- Start the kube-bind server at `http://kube-bind.dev.local:8080`

After the development environment is ready, you can:

1. **Authenticate to the provider cluster:**
   ```bash
   kubectl bind login http://kube-bind.dev.local:8080
   ```
   This will open a browser for authentication and save the configuration.

2. **Bind API services from provider to consumer:**
   ```bash
   KUBECONFIG=kind-consumer.kubeconfig kubectl bind --konnector-host-alias <docker-internal-ip-address>:kube-bind.dev.local
   ```
   This opens the kube-bind web UI where you can select and bind API services.

3. **Verify bound resources:**
   ```bash
   KUBECONFIG=kind-consumer.kubeconfig kubectl get crd
   KUBECONFIG=kind-consumer.kubeconfig kubectl get apiservicebindings -n kube-bind
   ```

4. **Create example resource**
   ```bash
   kubectl bind dev example | KUBECONFIG=kind-consumer.kubeconfig kubectl create -f -
   ```

5. **Check resource synced to provider**
   ```bash
   KUBECONFIG=kind-provider.kubeconfig kubectl get mangodbs.mangodb.com -A
   ```

### Production Deployment

For production deployments, you can deploy a kube-bind backend using helm:

- **[Using Helm Chart](helm.md)** - Recommended for production deployments

Once you have a kube-bind backend running (either development or production), you can connect to it:

### Connect to kube-bind Server

```bash
kubectl bind login https://my-kube-bind-server.example.com
``` 

### Open kube-bind Web UI and bind services

```bash
kubectl bind
```

