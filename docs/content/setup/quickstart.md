---
description: >
  Get started with kube bind.
---

# Quickstart

## Prerequisites

- [kubectl](https://kubernetes.io/docs/tasks/tools/#kubectl)
- [kube-bind CLI](kubectl-plugin.md) installed

## Start with kube-bind

### Deploy a kube-bind Backend

You can deploy a kube-bind backend using helm (recommended):

- **[Using Helm Chart](helm.md)** - Recommended for production deployments

## Once you have a kube-bind backend running, you can connect to it using the `kubectl bind` plugin.

### Connect to kube-bind Server

```bash
kubectl bind login https://my-kube-bind-server.example.com
``` 

### Open kube-bind Web UI and bind services

```bash
kubectl bind
```

