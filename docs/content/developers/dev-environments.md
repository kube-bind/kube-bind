---
description: >
  How to setup a development environment for contributing to kube-bind.
title: Developer Guide
---

# Development Environment

!!! note
    There are multiple ways to set up a development environment for kube-bind. This guide outlines one of the common approaches using `kind` and `kcp`. You can adapt these instructions based on your preferences and existing setups.

Due to the fact that kube-bind is by nature a multi-cluster system, for development purposes it's recommended to have multiple clusters running or use kcp to simulate multiple clusters. Below are instructions for both approaches.

All the instructions assume you have already cloned the kube-bind repository and have Go installed.

=== "kcp"

    kcp requires initial setup to be run before it can be used.
    This includes setting up workspace/provider and setting up all the APIResourceSchemas and APIExports.

    It has its own Go module to avoid kcp dependencies in the main kube-bind module.

    This is not required if you are doing deeper integration, and controlling the setup with your own scripts.

    It's good to have the kcp CLI installed to help with workspace management:

    ```bash
    kubectl krew index add kcp-dev https://github.com/kcp-dev/krew-index.git
    kubectl krew install kcp-dev/kcp
    kubectl krew install kcp-dev/ws
    kubectl krew install kcp-dev/create-workspace
    ```

    # How to run

    ## Preparation

    1. Start kcp

    ```bash
    make run-kcp
    ```

    ## Backend

    3. Bootstrap kcp. This is a dedicated step to set up kcp with required workspaces and APIExports for kube-bind:
    ```bash
    cp .kcp/admin.kubeconfig .kcp/backend.kubeconfig
    export KUBECONFIG=.kcp/backend.kubeconfig
    ./bin/kcp-init --kcp-kubeconfig $KUBECONFIG
    ```
    4. Run the backend:
    ```
    k ws use :root:kube-bind

    ./bin/backend \
    --multicluster-runtime-provider kcp \
    --server-url=$(kubectl get apiexportendpointslice kube-bind.io -o jsonpath="{.status.endpoints[0].url}") \
    --oidc-issuer-url=http://127.0.0.1:8080/oidc \
    --oidc-callback-url=http://127.0.0.1:8080/api/callback \
    --oidc-type=embedded \
    --pretty-name="BigCorp.com" \
    --namespace-prefix="kube-bind-" \
    --schema-source apiresourceschemas \
    --consumer-scope=cluster
    ```

    This process will keep running, so open a new terminal.

    ## Provider

    5. Copy the kubeconfig to the provider and create provider workspace:
    ```bash
    cp .kcp/admin.kubeconfig .kcp/provider.kubeconfig
    export KUBECONFIG=.kcp/provider.kubeconfig
    k ws use :root
    kubectl create-workspace provider --enter
    ```

    6. Bind the APIExport to the provider workspace
    ```bash
    kubectl kcp bind apiexport root:kube-bind:kube-bind.io \
    --accept-permission-claim clusterrolebindings.rbac.authorization.k8s.io \
    --accept-permission-claim clusterroles.rbac.authorization.k8s.io \
    --accept-permission-claim customresourcedefinitions.apiextensions.k8s.io \
    --accept-permission-claim serviceaccounts.core \
    --accept-permission-claim configmaps.core \
    --accept-permission-claim secrets.core \
    --accept-permission-claim subjectaccessreviews.authorization.k8s.io \
    --accept-permission-claim namespaces.core \
    --accept-permission-claim roles.rbac.authorization.k8s.io \
    --accept-permission-claim rolebindings.rbac.authorization.k8s.io \
    --accept-permission-claim apiresourceschemas.apis.kcp.io
    ```

    7. Create CRD in provider:
    ```bash
    kubectl apply -f contrib/kcp/deploy/examples/apiexport.yaml
    kubectl apply -f contrib/kcp/deploy/examples/apiresourceschema-cowboys.yaml
    kubectl apply -f contrib/kcp/deploy/examples/apiresourceschema-sheriffs.yaml
    kubectl kcp bind apiexport root:provider:cowboys-stable

    kubectl apply -f deploy/examples/template-cowboys.yaml
    kubectl apply -f deploy/examples/template-sheriffs.yaml
    kubectl apply -f deploy/examples/collection.yaml
    ```

    8. Get LogicalCluster:

    ```bash
    kubectl get logicalcluster
    # NAME      PHASE   URL                                                    AGE
    # cluster   Ready   https://192.168.2.166:6443/clusters/2myqz7lt9i0u5kzb
    ```

    ## Consumer

    9. Now we gonna initiate consumer:
    ```bash
    cp .kcp/admin.kubeconfig .kcp/consumer.kubeconfig
    export KUBECONFIG=.kcp/consumer.kubeconfig
    kubectl ws use :root
    kubectl ws create consumer --enter
    ```

    10. Bind the thing:

    ```bash
    ./bin/kubectl-bind login http://127.0.0.1:8080 --cluster 2uoiu445cp94dkrz
    ./bin/kubectl-bind --dry-run -o yaml > apiserviceexport.yaml

    # Extract secret for binding process. Note that secret name is not the same as output from command above. Check secret
    # name by running `kubectl get secret -n kube-bind`
    kubectl get secrets -n kube-bind -o jsonpath='{.items[0].data.kubeconfig}' | base64 -d > remote.kubeconfig

    namespace=$(yq '.contexts[0].context.namespace' remote.kubeconfig)

    ./bin/kubectl-bind apiservice -v 6 --remote-kubeconfig remote.kubeconfig -f apiserviceexport.yaml --skip-konnector --remote-namespace "$namespace"
    ```

    This will keep running, so switch to a new terminal.

    ### Consumer Konnector

    Start konnector:

    ```bash
    ./bin/konnector --lease-namespace default --kubeconfig .kcp/consumer.kubeconfig
    ```

    Create example resources in consumer:
    ```bash
    kubectl apply -f deploy/examples/cr-cowboy.yaml
    kubectl apply -f deploy/examples/cr-sheriff.yaml
    ```

=== "Kind"

    This guide will walk you through setting up kube-bind between two Kubernetes clusters, where

    * **Provider cluster**:

    * Deploys kube-bind/backend
    * Provides kube-bind compatible backend for Sheriffs resources

    * **Consumer cluster**:

    * Provides an application consuming Sheriffs and Cowboys resources from the Provider cluster

    This guide works best on Linux. macOS and Windows users may need to adjust some commands accordingly.

    ## Pre-requisites

    To start, you'll need the following tools available in your system or a VM:

    * [`kind`](https://kind.sigs.k8s.io/docs/user/quick-start/#installation)
    * [`kubectl`](https://kubernetes.io/docs/tasks/tools/)
    * [`kubectl-bind`](https://github.com/kube-bind/kube-bind/releases/latest) (a kubectl plugin)
    * [`jq`](https://jqlang.github.io/jq/download/)
    * port 8080 and 6443 open on your localhost

    To install `kubectl-bind` plugin, please download the archive for your platform from the link above, extract it, and place the `kubectl-bind` executable in your system's `$PATH`.

    > Tip: In case of encountering `Too many open files` error when deploying the Kind clusters, run following commands:
    >
    > ```sh
    > sudo sysctl fs.inotify.max_user_watches=524288
    > sudo sysctl fs.inotify.max_user_instances=512
    > ```
    >
    > See the [kind documentation](https://kind.sigs.k8s.io/docs/user/known-issues/#pod-errors-due-to-too-many-open-files) for more details.

    ## Setup

    run `kubectl bind dev create`, which will automatically create two Kind clusters (provider and consumer), deploy kube-bind backend in the provider cluster, and print required commands to bind the consumer cluster to the provider.

    This command will create two Kind clusters named `kind-provider` and `kind-consumer`, set up a Docker network for them to communicate, and install kube-bind backend in the provider cluster and establish right `hostAlias` entries for communication between clusters.

    ```sh
    kubectl bind dev create
    ```

    You should see output similar to this:

    ```sh
    kubectl bind dev create                                                                              09:59:12
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
        Note /etc/hosts modification and 2 commands to run to bind the consumer cluster.

    ## Login to Provider cluster

    This should give you UI authorization screen.

    ```bash
    kubectl bind login http://kube-bind.dev.local:8080
    ```

    # Bind first provider API service

    ```bash
    KUBECONFIG=kind-consumer.kubeconfig kubectl bind --konnector-host-alias 192.168.155.2:kube-bind.dev.local
    ```

    Now in the consumer cluster you should see these CRDs:

    ```bash
    kubectl get crd                                                                                        10:10:56
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
