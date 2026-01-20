---
title: CloudNativePG
description: |
    Guide on integrating kube-bind with CloudNativePG for automated Postgres database management.
weight: 30
---

# CloudNativePG Integration

This document shows how the [CloudNativePG](https://cloudnative-pg.io/) Postgres database operator
can be integrated and provided using kube-bind.

## Setup

The following sections will guide you through the one-time setup that is required for providing
Postgres databases using CloudNativePG and kube-bind.

### Install CloudNativePG

Install CloudNativePG in your Kubernetes cluster, where kube-bind backend is running, if you
haven't already. Follow the [official installation guide](https://cloudnative-pg.io/docs/1.28/installation_upgrade)
in the CloudNativePG documentation. In its simplest form, the installation consists of applying this
manifest:

```bash
kubectl apply \
  --server-side \
  --filename https://raw.githubusercontent.com/cloudnative-pg/cloudnative-pg/release-1.28/releases/cnpg-1.28.0.yaml
```

### Export the Cluster CRD

To export the `Cluster` CRD in the provider cluster, add the kube-bind export label to it:

```bash
kubectl label crd clusters.postgresql.cnpg.io kube-bind.io/exported=true --overwrite
```

### Create a APIServiceExportTemplate

It's now time to configure kube-bind to export the `clusters` resource. To do so, create a
kube-bind `APIServiceExportTemplate` in the provider cluster like this one:

```yaml
kubectl apply -f - <<EOF
apiVersion: kube-bind.io/v1alpha2
kind: APIServiceExportTemplate
metadata:
  labels:
    provider: cloudnativepg
  name: pg-clusters
spec:
  permissionClaims:
  - group: ""
    resource: secrets
    selector:
      references:
        - resource: clusters
          group: postgresql.cnpg.io
          jsonPath:
            name: 'spec.bootstrap.initdb.secret.name'
  resources:
  - group: postgresql.cnpg.io
    resource: clusters
    versions:
    - v1
  scope: Namespaced
EOF
```

## Usage

Now that everything is set up, users can begin to bind to your backend and begin consuming the new
API.

### Login to kube-bind

```bash
kubectl bind login https://kube-bind.example.com
```

### Request a Binding

Request a binding to the `pg-clusters` template created above. This will allow you to create
`Database` objects in your consumer cluster.

**NB::** Make sure you've set your `KUBECONFIG` to the consumer cluster.

```bash
# you will get redirected to UI to authenticate and pick the template
kubectl bind
```

!!! note
    If you're using one of the [dev environments](../../developers/dev-environment/index.md), you
    might need to add additional arguments to the `bind` command, like `--konnector-host-alias`.

### Wait for the Binding to be Established

Once the binding is active, you can create `Cluster` object in your consumer cluster,
and you will get `Cluster` objects synced from the provider cluster.

```bash
kubectl bind
🌐 Opening kube-bind UI in your browser...
    https://kube-bind.example.com?redirect_url=....

Browser opened successfully
Waiting for binding completion from UI...
   (Press Ctrl+C to cancel)

Binding completed successfully!
🔒 Updated secret kube-bind/kubeconfig-zxrdn for host https://kube-bind.example.com, namespace kube-bind-bp52k
🚀 Deploying konnector v0.6.0 to namespace kube-bind.
✅ Created APIServiceBinding pg-clusters-pk5c8 for 1 resources
Created 1 APIServiceBinding(s):
  - pg-clusters-pk5c8
Resources bound successfully!
```

### Create a Managed Database

Verify that a `clusters.postgresql.cnpg.io` CRD is synced to the consumer cluster:

```bash
k get crd clusters.postgresql.cnpg.io
NAME                           CREATED AT
clusters.postgresql.cnpg.io    2025-11-27T14:22:18Z
```

Order a new consumer database instance by creating a Postgres cluster. We need to provide our own
credentials, otherwise the automatically generated credentials on the provider cluster will be
inaccessible to consumers.

```yaml
kubectl apply -f - <<EOF
apiVerson: v1
kind: Secret
metadata:
  name: cluster-example-app-credentials
data:
  username: bXktYXBwbGljYXRpb24=
  password: c3VwZXItc2VjcjN0LXBhc3N3MHJk

---
apiVersion: postgresql.cnpg.io/v1
kind: Cluster
metadata:
  name: cluster-example
spec:
  instances: 3
  storage:
    size: 1Gi
  bootstrap:
    initdb:
      secret:
        name: cluster-example-app-credentials
EOF
```

### Wait for Provisioning

The kube-bind konnector and the CloudNativePG operator should now be busy provisioning your
database. You can observe the provisioned database and connection Secret in the provider cluster:

```bash
kubectl -n kube-bind-xbrxn-default get clusters

NAME              AGE   INSTANCES   READY   STATUS                     PRIMARY
cluster-example   22m   3           3       Cluster in healthy state   cluster-example-1
```

```bash
kubectl -n kube-bind-xbrxn-default get clusters cluster-example -o yaml
```

```yaml
apiVersion: postgresql.cnpg.io/v1
kind: Cluster
metadata:
  annotations:
    kube-bind.io/cluster-namespace: kube-bind-xbrxn
    kubectl.kubernetes.io/last-applied-configuration: |
      {"apiVersion":"postgresql.cnpg.io/v1","kind":"Cluster","metadata":{"annotations":{},"name":"cluster-example","namespace":"default"},"spec":{"bootstrap":{"initdb":{"secret":{"name":"cluster-example-app-credentials"}}},"instances":3,"storage":{"size":"1Gi"}}}
  creationTimestamp: "2026-01-22T10:13:57Z"
  generation: 1
  name: cluster-example
  namespace: kube-bind-xbrxn-default
  resourceVersion: "4329"
  uid: a8949c70-c3b7-417f-9191-fb43cbd2d667
spec:
  affinity:
    podAntiAffinityType: preferred
  bootstrap:
    initdb:
      database: app
      encoding: UTF8
      localeCType: C
      localeCollate: C
      owner: app
      secret:
        name: cluster-example-app-credentials
  enablePDB: true
  enableSuperuserAccess: false
  failoverDelay: 0
  imageName: ghcr.io/cloudnative-pg/postgresql:18.1-system-trixie
  instances: 3
  logLevel: info
  maxSyncReplicas: 0
  minSyncReplicas: 0
  monitoring:
    customQueriesConfigMap:
    - key: queries
      name: cnpg-default-monitoring
    disableDefaultQueries: false
    enablePodMonitor: false
  postgresGID: 26
  postgresUID: 26
  postgresql:
    parameters:
      archive_mode: "on"
      archive_timeout: 5min
      dynamic_shared_memory_type: posix
      full_page_writes: "on"
      log_destination: csvlog
      log_directory: /controller/log
      log_filename: postgres
      log_rotation_age: "0"
      log_rotation_size: "0"
      log_truncate_on_rotation: "false"
      logging_collector: "on"
      max_parallel_workers: "32"
      max_replication_slots: "32"
      max_worker_processes: "32"
      shared_memory_type: mmap
      shared_preload_libraries: ""
      ssl_max_protocol_version: TLSv1.3
      ssl_min_protocol_version: TLSv1.3
      wal_keep_size: 512MB
      wal_level: logical
      wal_log_hints: "on"
      wal_receiver_timeout: 5s
      wal_sender_timeout: 5s
    syncReplicaElectionConstraint:
      enabled: false
  primaryUpdateMethod: restart
  primaryUpdateStrategy: unsupervised
  probes:
    liveness:
      isolationCheck:
        connectionTimeout: 1000
        enabled: true
        requestTimeout: 1000
  replicationSlots:
    highAvailability:
      enabled: true
      slotPrefix: _cnpg_
    synchronizeReplicas:
      enabled: true
    updateInterval: 30
  resources: {}
  smartShutdownTimeout: 180
  startDelay: 3600
  stopDelay: 1800
  storage:
    resizeInUseVolumes: true
    size: 1Gi
  switchoverDelay: 3600
status:
  availableArchitectures:
  - goArch: amd64
    hash: 527e2e3b680dfba7f7578b98530f755f36156e7be00acda3318ef36fe4f4418f
  - goArch: arm64
    hash: aaa74c6061f3fe30f230a664b4f6076bb886c9a89d4eba3293483525c5cff533
  certificates:
    clientCASecret: cluster-example-ca
    expirations:
      cluster-example-ca: 2026-04-22 10:08:57 +0000 UTC
      cluster-example-replication: 2026-04-22 10:08:57 +0000 UTC
      cluster-example-server: 2026-04-22 10:08:57 +0000 UTC
    replicationTLSSecret: cluster-example-replication
    serverAltDNSNames:
    - cluster-example-rw
    - cluster-example-rw.kube-bind-xbrxn-default
    - cluster-example-rw.kube-bind-xbrxn-default.svc
    - cluster-example-rw.kube-bind-xbrxn-default.svc.cluster.local
    - cluster-example-r
    - cluster-example-r.kube-bind-xbrxn-default
    - cluster-example-r.kube-bind-xbrxn-default.svc
    - cluster-example-r.kube-bind-xbrxn-default.svc.cluster.local
    - cluster-example-ro
    - cluster-example-ro.kube-bind-xbrxn-default
    - cluster-example-ro.kube-bind-xbrxn-default.svc
    - cluster-example-ro.kube-bind-xbrxn-default.svc.cluster.local
    serverCASecret: cluster-example-ca
    serverTLSSecret: cluster-example-server
  cloudNativePGCommitHash: a9696201f
  cloudNativePGOperatorHash: 527e2e3b680dfba7f7578b98530f755f36156e7be00acda3318ef36fe4f4418f
  conditions:
  - lastTransitionTime: "2026-01-22T10:30:08Z"
    message: A single, unique system ID was found across reporting instances.
    reason: Unique
    status: "True"
    type: ConsistentSystemID
  - lastTransitionTime: "2026-01-22T10:31:06Z"
    message: Cluster is Ready
    reason: ClusterIsReady
    status: "True"
    type: Ready
  - lastTransitionTime: "2026-01-22T10:30:08Z"
    message: Continuous archiving is working
    reason: ContinuousArchivingSuccess
    status: "True"
    type: ContinuousArchiving
  configMapResourceVersion:
    metrics:
      cnpg-default-monitoring: "2120"
  currentPrimary: cluster-example-1
  currentPrimaryTimestamp: "2026-01-22T10:30:07.986099Z"
  healthyPVC:
  - cluster-example-1
  - cluster-example-2
  - cluster-example-3
  image: ghcr.io/cloudnative-pg/postgresql:18.1-system-trixie
  instanceNames:
  - cluster-example-1
  - cluster-example-2
  - cluster-example-3
  instances: 3
  instancesReportedState:
    cluster-example-1:
      ip: 10.244.0.21
      isPrimary: true
      timeLineID: 1
    cluster-example-2:
      ip: 10.244.0.24
      isPrimary: false
      timeLineID: 1
    cluster-example-3:
      ip: 10.244.0.27
      isPrimary: false
      timeLineID: 1
  instancesStatus:
    healthy:
    - cluster-example-1
    - cluster-example-2
    - cluster-example-3
  latestGeneratedNode: 3
  managedRolesStatus: {}
  pgDataImageInfo:
    image: ghcr.io/cloudnative-pg/postgresql:18.1-system-trixie
    majorVersion: 18
  phase: Cluster in healthy state
  poolerIntegrations:
    pgBouncerIntegration: {}
  pvcCount: 3
  readService: cluster-example-r
  readyInstances: 3
  secretsResourceVersion:
    applicationSecretVersion: "3969"
    clientCaSecretVersion: "2089"
    replicationSecretVersion: "2092"
    serverCaSecretVersion: "2089"
    serverSecretVersion: "2090"
  switchReplicaClusterStatus: {}
  systemID: "7598131281368047651"
  targetPrimary: cluster-example-1
  targetPrimaryTimestamp: "2026-01-22T10:13:58.221403Z"
  timelineID: 1
  topology:
    instances:
      cluster-example-1: {}
      cluster-example-2: {}
      cluster-example-3: {}
    nodesUsed: 1
    successfullyExtracted: true
  writeService: cluster-example-rw
```

---

For troubleshooting and more information, check the [kube-bind documentation](https://kube-bind.io/docs/).
