# Backend-Only Bindings

**Status**: Implemented  
**Date**: January 2025  

## Problem

Traditional kube-bind requires OIDC authentication and manual web UI interaction to bind services. This doesn't work for:
- CI/CD pipelines and automation
- Headless servers
- GitOps workflows

## Solution

**APIServiceBindingBundle**: A single resource that automatically discovers and binds ALL services from a provider cluster using only a kubeconfig.

**One command instead of a multi-step handshake:**

```bash
kubectl create secret generic provider-kubeconfig \
  --from-file=kubeconfig=provider.kubeconfig

kubectl apply -f - <<EOF
apiVersion: kube-bind.io/v1alpha2
kind: APIServiceBindingBundle
metadata:
  name: prod-services
spec:
  kubeconfigSecretRef:
    name: provider-kubeconfig
    namespace: default
    key: kubeconfig
EOF
```

## How It Works

1. **ServiceBindingBundle controller** watches for bundle resources
2. **Validates** the kubeconfig secret
3. **Polls provider** cluster every 15s for APIServiceExports
4. **Auto-creates** APIServiceBinding for each discovered export
5. **Auto-deletes** bindings when exports disappear

**Key features:**
- Uses OwnerReferences for automatic garbage collection
- Configurable polling interval (`--provider-polling-interval=15s`)
- Namespace extracted from kubeconfig context
- No watch setup needed - simple polling

## Usage

### Multi-Provider

```yaml
apiVersion: kube-bind.io/v1alpha2
kind: APIServiceBindingBundle
metadata:
  name: prod-provider
spec:
  kubeconfigSecretRef:
    name: prod-kubeconfig
    namespace: default
    key: kubeconfig
---
apiVersion: kube-bind.io/v1alpha2
kind: APIServiceBindingBundle
metadata:
  name: staging-provider
spec:
  kubeconfigSecretRef:
    name: staging-kubeconfig
    namespace: default
    key: kubeconfig
```

### Check Status

```bash
kubectl get apiservicebindingbundles
kubectl get apiservicebindings  # Auto-created bindings
```

## Implementation Summary

- **~950 lines** of new controller code
- **Polling-based**: Simple 15s ticker (configurable)
- **OwnerReferences**: Automatic cleanup when bundle is deleted
- **Backward compatible**: Existing OIDC flows work unchanged

## Comparison

| Aspect | Traditional | Backend-Only |
|--------|-------------|--------------|
| Authentication | OIDC browser | Kubeconfig |
| Discovery | Manual UI | Automatic polling |
| Scope | Per-service | All services |
| Automation | Difficult | Native |
| GitOps | No | Yes |

## Files Changed

**New Controllers:**
- `pkg/konnector/controllers/servicebindingbundle/` (620 lines)
- `pkg/indexers/servicebindingbundle.go` (39 lines)

**New API:**
- `sdk/apis/kubebind/v1alpha2/apiservicebindingbundle_types.go` (83 lines)
- `deploy/crd/kube-bind.io_apiservicebindingbundles.yaml` (149 lines)

**Modified:**
- `pkg/konnector/` - Integrate new controller
- `pkg/konnector/options/` - Add `--provider-polling-interval` flag
- Generated client code (~15 files)

