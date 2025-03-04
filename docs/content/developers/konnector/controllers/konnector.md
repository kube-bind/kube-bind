# konnector

The konnector implements the main reconciliation loop and watches `APIServiceBindings` and the referenced `Secrets` in the **consumer cluster**.

It is responsible for:

* starting / stopping a set of controllers per service provider

## Overview

```mermaid
flowchart TD
    %% Nodes
    start@{ shape: start }
    stop@{ shape: stop }

    enqueue_reconcile(["Enqueue reconcile call for APIServiceBinding"])

    get_kubeconfig_secret(["Get referenced kubeconfig secret"])
    is_kubeconfig_empty{"kubeconfig<br>empty?"}

    get_controller_context_for_binding(["Get controller context for binding"])
    is_controller_context_for_binding_present{"context<br>exists?"}
    is_controller_context_current{"context<br>up-to-date?"}
    is_controller_context_used{"context<br>in use?"}

    get_controller_context_for_kubeconfig(["Get controller context for kubeconfig"])
    is_controller_context_for_kubeconfig_present{"context<br>exists?"}

    create_controller_context_for_binding(["Create controller context for binding"])
    add_binding_to_controller_context(["Add binding to controller context"])
    remove_binding_from_controller_context(["Remove binding from controller context"])

    start_controllers(["Start controllers"])
    stop_controllers(["Stop controllers"])

    %% Transitions
    start --> enqueue_reconcile
    enqueue_reconcile --> get_kubeconfig_secret
    get_kubeconfig_secret --> get_controller_context_for_binding
    get_controller_context_for_binding --> is_controller_context_for_binding_present

    is_controller_context_for_binding_present -->|yes| is_controller_context_current
        is_controller_context_current -->|yes| is_kubeconfig_empty
        is_controller_context_current -->|no| remove_binding_from_controller_context
            remove_binding_from_controller_context --> is_controller_context_used
                is_controller_context_used -->|yes| stop
                is_controller_context_used -->|no| stop_controllers
                    stop_controllers --> stop

    is_controller_context_for_binding_present -->|no| is_kubeconfig_empty
        is_kubeconfig_empty -->|yes| stop
        is_kubeconfig_empty -->|no| get_controller_context_for_kubeconfig
            get_controller_context_for_kubeconfig --> is_controller_context_for_kubeconfig_present
            is_controller_context_for_kubeconfig_present -->|yes| add_binding_to_controller_context
                add_binding_to_controller_context --> stop
            is_controller_context_for_kubeconfig_present -->|no| create_controller_context_for_binding
                create_controller_context_for_binding --> start_controllers
                start_controllers --> stop
```
