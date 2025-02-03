# APIServiceNamespaces

The APIServiceNamespace controller watches `Namespaces` in the **consumer cluster** and `APIServiceNamespaces` in the **provider cluster**.

It is responsible for:

* synchronizing `Namespaces` in the **consumer cluster** with `APIServiceNamespaces` in the **provider cluster**

## Overview

```mermaid
flowchart TD
    %% Nodes
    start@{ shape: start }
    stop@{ shape: stop }

    enqueue_reconcile(["Enqueue reconcile call for APIServiceNamespace"])

    get_namespace(["Get associated namespace"])
    is_namespace_present(["namespace<br>exists?"])

    delete_api_service_namespace(["Delete APIServiceNamespace"])

    %% Transitions
    start --> enqueue_reconcile
    enqueue_reconcile --> get_namespace
    get_namespace --> is_namespace_present

    is_namespace_present --> |yes| stop
    is_namespace_present --> |no| delete_api_service_namespace
    delete_api_service_namespace --> stop
```
