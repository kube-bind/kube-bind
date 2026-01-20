---
description: >
  How to setup a development environment for contributing to kube-bind.
title: kind
---

# Development Environment using kind

All the instructions assume you have already cloned the kube-bind repository and have Go installed.

This guide will walk you through setting up kube-bind between two Kubernetes clusters, where

* **Provider cluster**:
    * Deploys kube-bind backend
    * Provides kube-bind compatible backend for MangoDB resources
* **Consumer cluster**:
    * Provides an application consuming MangoDBs from the provider cluster

This guide works best on Linux. macOS and Windows users may need to adjust some commands accordingly.

## Pre-requisites

To start, you'll need the following tools available in your system:

* [`kind`](https://kind.sigs.k8s.io/docs/user/quick-start/#installation)
* [`kubectl`](https://kubernetes.io/docs/tasks/tools/)
* [`kubectl-bind`](https://github.com/kube-bind/kube-bind/releases/latest) (a kubectl plugin)
* [`jq`](https://jqlang.github.io/jq/download/)
* port 8080 and 6443 open on your localhost

To install `kubectl-bind` plugin, please download the archive for your platform from the link above,
extract it, and place the `kubectl-bind` executable in your system's `$PATH`.

!!! note
    **Tip:** In case of encountering `Too many open files` error when deploying the Kind clusters,
    run following commands:

    ```sh
    sudo sysctl fs.inotify.max_user_watches=524288
    sudo sysctl fs.inotify.max_user_instances=512
    ```

    See the [kind documentation](https://kind.sigs.k8s.io/docs/user/known-issues/#pod-errors-due-to-too-many-open-files)
    for more details.

## Setup

Run `kubectl bind dev create`, which will automatically create two Kind clusters (provider and consumer),
deploy kube-bind backend in the provider cluster, and print required commands to bind the consumer
cluster to the provider.

This command will create two Kind clusters named `kind-provider` and `kind-consumer`, set up a
Docker network for them to communicate, and install kube-bind backend in the provider cluster and
establish right `hostAlias` entries for communication between clusters.

```sh
kubectl bind dev create
```

You should see output similar to this:

```sh
kubectl bind dev create
🧪 EXPERIMENTAL: kube-bind dev command is in preview
📦 Requirements: Docker must be installed and running
Warning: Could not automatically add host entry. Please run:
echo '127.0.0.1 kube-bind.dev.local' | sudo tee -a /etc/hosts
Kind cluster kind-provider already exists, skipping creation
Wrote kubeconfig kind-provider.kubeconfig
Helm chart installed successfully
Provider cluster IP address: 192.168.155.2
Creating kind cluster kind-consumer with network kube-bind-dev
Kind cluster kind-consumer created
Wrote kubeconfig kind-consumer.kubeconfig
🚀 kube-bind dev environment is ready!

- Provider cluster kubeconfig: kind-provider.kubeconfig
- Consumer cluster kubeconfig: kind-consumer.kubeconfig
- kube-bind server URL: http://kube-bind.dev.local:8080
- Next steps:
1. Run login to authenticate to the provider cluster:

kubectl bind login http://kube-bind.dev.local:8080

2. Run bind to bind an API service from the provider to the consumer cluster:

KUBECONFIG=kind-consumer.kubeconfig kubectl bind --konnector-host-alias 192.168.155.2:kube-bind.dev.local
```

!!! note
    Note `/etc/hosts` modification and 2 commands to run to bind the consumer cluster.

## Login to Provider Cluster

This should give you UI authorization screen.

```bash
kubectl bind login http://kube-bind.dev.local:8080
```

## Bind First Provider API Service

```bash
export KUBECONFIG=kind-consumer.kubeconfig
kubectl bind --konnector-host-alias 192.168.155.2:kube-bind.dev.local
```

Now in the consumer cluster you should see these CRDs:

```bash
kubectl get crd
NAME                              CREATED AT
apiservicebindings.kube-bind.io   2025-11-12T08:10:48Z
mangodbs.mangodb.com              2025-11-12T08:10:56Z
```

Try creating an example MangoDB resource and observe it being synced to provider clusters:

```bash
kubectl bind dev example | kubectl create -f -
KUBECONFIG=kind-consumer.kubeconfig kubectl get mangodbs.mangodb.com
KUBECONFIG=kind-provider.kubeconfig kubectl get mangodbs.mangodb.com
```

## Cleanup

```sh
kind bind dev delete
```
