# Cluster Binding

## Overview

```mermaid
sequenceDiagram
    autonumber
    participant consumer-cluster as Consumer cluster
    participant client as Client
    participant provider-backend as Provider backend
    participant authentication-provider as Authentication Provider

    %% Get provider information
    client->>+provider-backend: GET "${PROVIDER_BINDING_URL}"
    provider-backend->>-client: 200 "BindingProvider"

    client->>client: Verify "BindingProvider"

    %% Authenticate to provider
    client-->>authentication-provider: Authenticate to provider

    %% Bind to API
    provider-backend->>client: 200 "BindingResponse"

    client->>consumer-cluster: Ensure kubeconfig secret

    loop For each "BindingResponse.requests"
    client->>consumer-cluster: Bind remote API to consumer cluster
    end
```

## Contracts

**BindingResponse**

In case the cluster binding request is accepted, the provider backend **must** ensure that

* the kubeconfig returned in the `BindingResponse` contains a current context
* the configured current context points to the "cluster namespace"
* the `ClusterBinding` object named "cluster" exists in the "cluster namespace"
* the secret referenced by `ClusterBinding.spec.kubeconfigSecretRef.name` exists
* the key of the secret referenced by `ClusterBinding.spec.kubeconfigSecretRef.key` contains a valid kubeconfig
* the configured current context points to a user with at least the following permissions in the "cluster namespace" (here expressed as RBAC rules)
  ```yaml
  - apiGroups: ["kube-bind.io"]
    resources: ["apiserviceexportrequests"]
    verbs: ["create", "delete", "patch", "update", "get", "list", "watch"]

  - apiGroups: ["kube-bind.io"]
    resources: ["apiserviceexports"]
    verbs: ["get", "watch", "list"]
  - apiGroups: ["kube-bind.io"]
    resources: ["apiserviceexports/status"]
    verbs: ["get", "patch", "update"]

  - apiGroups: ["kube-bind.io"]
    resources: ["apiservicenamespaces"]
    verbs: ["create", "delete", "patch", "update", "get", "list", "watch"]

  - apiGroups: ["kube-bind.io"]
    resources: ["clusterbindings"]
    verbs: ["get", "watch", "list"]
  - apiGroups: ["kube-bind.io"]
    resources: ["clusterbindings/status"]
    verbs: ["get", "patch", "update"]

  - apiGroups: [""]
    resources: ["secrets"]
    verbs: ["get", "watch", "list"]
  ```
