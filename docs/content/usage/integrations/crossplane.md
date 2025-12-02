---
title: Crossplane
description: |
    Guide on integrating kube-bind with Crossplane for managed database provisioning.
weight: 20
---

# Crossplane Integration

This document provides an example deployment walkthrough showing how to integrate kube-bind with Crossplane and how to deploy a sample managed MySQL resource using two kind clusters: a provider cluster (where Crossplane runs and kube-bind backend to export APIs) and a consumer cluster (which allows to bind those APIs using kube-bind konnector).

1. **Install Crossplane** in your Kubernetes cluster where the kube-bind backend will run.
   You can follow the official installation guide [here](https://crossplane.io/docs/v1.14/getting-started/install-configure.html).

```bash
    helm repo add crossplane-stable https://charts.crossplane.io/stable
    helm repo update

    helm install crossplane crossplane-stable/crossplane \
    --namespace crossplane-system \
    --create-namespace
```

2. **Install a Crossplane provider-sql and set up the ProviderConfig.**

   In the example, we will set up mysql database:

```yaml
kubectl apply -f - <<EOF
apiVersion: pkg.crossplane.io/v1
kind: Provider
metadata:
    name: provider-sql
spec:
    package: xpkg.upbound.io/crossplane-contrib/provider-sql:v0.12.0
EOF
```


    Create a secret and ProviderConfig:

```yaml
kubectl apply -f - <<EOF
apiVersion: mysql.sql.crossplane.io/v1alpha1
kind: ProviderConfig
metadata:
  name: default
spec:
  credentials:
    source: MySQLConnectionSecret
    connectionSecretRef:
      namespace: mysql
      name: db-conn
  tls: preferred
EOF
```

```bash
kubectl create secret generic db-conn --from-literal endpoint=mysql.default.svc.cluster.local --from-literal port=3306 --from-literal username=root --from-literal password=password
```

3. **Setup the mysql deployment in the provider cluster**

    Create and setup Deployment, PersistentVolume, PersistentVolumeClaim and Service for MySQL instance

```bash
kubectl apply -f hack/crossplane-example/mysql
```

4. **Create a Crossplane XRD and Composition for a managed MySQL database**

    Apply both manifests:

```bash
kubectl apply -f hack/crossplane-example/xrd
```

5. **Export the database API using kube-bind.**
   Create an APIServiceExportTemplate for the mysqldatabase.mangodb.com resource:

```yaml
kubectl apply -f - <<EOF
apiVersion: kube-bind.io/v1alpha2
kind: APIServiceExportTemplate
metadata:
  name: mysqldatabase
spec:
  resources:
    - group: mangodb.com
      resource: mysqldatabases
      versions:
        - v1
  permissionClaims:
  - group: ""
    resource: secrets
    selector:
      references:
        - resource: users
          group: mysql.sql.crossplane.io
          jsonPath:
            name: 'spec.writeConnectionSecretToRef.name'
            namespace: 'spec.writeConnectionSecretToRef.namespace'
  scope: Cluster
EOF
```
    Apply it to the provider cluster:

```bash
kubectl apply -f hack/crossplane-example/db-apiserviceexport-database.yaml
 ```

6. **Login to kube-bind and request a binding to the exported database API.**

    ```bash
    kubectl bind login https://kube-bind.example.com
    # Authenticate and select the mysqldatabase export
    kubectl bind
    ```


7. **Wait for the binding to be established.** Once the binding is active, you can create `MySQLDatabase` resources in your consumer cluster, and you will get `MySQLDatabase` objects synced from the provider cluster.

```bash
kubectl bind
🌐 Opening kube-bind UI in your browser...
    https://kube-bind.example.com?redirect_url=....

Browser opened successfully
Waiting for binding completion from UI...
   (Press Ctrl+C to cancel)

Binding completed successfully!
🔒 Updated secret kube-bind/kubeconfig-zxrdn for host https://kube-bind.example.com, namespace kube-bind-bp52k
🚀 Deploying konnector v0.6.0 to namespace kube-bind with custom image "ghcr.io/kube-bind/konnector:v0.6.0".
✅ Created APIServiceBinding mysqldatabase-6rvjt for 1 resources
Created 1 APIServiceBinding(s):
  - mysqldatabase-6rvjt
Resources bound successfully!
```

8. **Create a managed database in your consumer cluster.**
    Verify that mysqldatabase.mongodb.com CRD is synced to the consumer cluster:
```bash
k get crd mysqldatabases.mangodb.com
NAME                         CREATED AT
mysqldatabases.mangodb.com   2025-11-27T14:22:18Z
```
    Order a new consumer-database instance in the provider cluster

```yaml
apiVersion: mangodb.com/v1
kind: MySQLDatabase
metadata:
  name: consumer-database
spec:
  name: consumer-database
```

```bash
 kubectl apply -f hack/crossplane-example/consumer-db.yaml
```

9. **Observe the provisioned database and connection secret in the provider cluster.**

```bash
kubectl get mysqldatabases.mangodb.com kube-bind-bp52k-consumer-database

NAME                                SYNCED   READY   COMPOSITION             AGE
kube-bind-bp52k-consumer-database   True     True    mysql-database-simple   18m
```

```bash
kubectl get secrets -n default
NAME                                                              TYPE                                DATA   AGE
consumer-database-connection-secret                               connection.crossplane.io/v1alpha1   4      18m
consumer-database-secret                                          Opaque                              1      18m
db-conn                                                           Opaque                              4      20m
kube-bind-tvq46-consumer-database-credentials                     Opaque                              4      18m
```

```bash
kubectl get mysqldatabases.mangodb.com kube-bind-bp52k-consumer-database -o yaml
```
```yaml
apiVersion: mangodb.com/v1
kind: MySQLDatabase
metadata:
  annotations:
    kube-bind.io/cluster-namespace: kube-bind-bp52k
    kubectl.kubernetes.io/last-applied-configuration: |
      {"apiVersion":"mangodb.com/v1","kind":"MySQLDatabase","metadata":{"annotations":{},"name":"consumer-database"},"spec":{"name":"consumer-database"}}
  creationTimestamp: "2025-11-270T15:39:31Z"
  finalizers:
  - composite.apiextensions.crossplane.io
  generation: 4
  labels:
    crossplane.io/composite: kube-bind-bp52k-consumer-database
  name: kube-bind-bp52k-consumer-database
  ownerReferences:
  - apiVersion: v1
    kind: Namespace
    name: kube-bind-bp52k
    uid: d20ee5be-e9d7-41e3-95ee-76897a554750
  resourceVersion: "136223"
  uid: 5d510666-f5d3-4bf5-98e0-384364a81170
spec:
  crossplane:
    compositionRef:
      name: mysql-database-simple
    compositionRevisionRef:
      name: mysql-database-simple-c36a727
    compositionUpdatePolicy: Automatic
    resourceRefs:
    - apiVersion: mysql.sql.crossplane.io/v1alpha1
      kind: Database
      name: consumer-database
    - apiVersion: mysql.sql.crossplane.io/v1alpha1
      kind: User
      name: consumer-database-user
    - apiVersion: v1
      kind: Secret
      name: consumer-database
      namespace: default
  name: consumer-database
status:
  conditions:
  - lastTransitionTime: "2025-11-27T15:39:32Z"
    observedGeneration: 4
    reason: ReconcileSuccess
    status: "True"
    type: Synced
  - lastTransitionTime: "2025-11-27T15:39:32Z"
    observedGeneration: 4
    reason: Available
    status: "True"
    type: Ready
  connectionSecret: consumer-database
  ready: true
```

You should see your MySQL instance created in the provider cluster and a secret with connection details, once Crossplane finish up provisioning of the database.

---

For troubleshooting and more information, check the [kube-bind documentation](https://kube-bind.io/docs/).
