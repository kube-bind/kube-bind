# APIServiceBindings

The APIServiceBinding controller watches `APIServiceBindings` and the referenced `Secrets` in the **consumer
cluster**.

It is responsible for:

* validating the kubeconfig stored in the secrets referenced by `APIServiceBindings`

## Overview

```mermaid
flowchart TD
    %% Nodes
    start@{ shape: start }
    stop@{ shape: stop }

    enqueue_reconcile(["Enqueue reconcile call for APIServiceBinding"])

    get_kubeconfig_secret(["Get referenced kubeconfig secret"])
    is_kubeconfig_valid{"kubeconfig<br>valid?"}

    set_condition_secret_valid_to_true(["Set condition 'SecretValid' to true"])
    set_condition_secret_valid_to_false(["Set condition 'SecretValid' to false"])

    %% Transitions
    start --> enqueue_reconcile
    enqueue_reconcile --> get_kubeconfig_secret
    get_kubeconfig_secret --> is_kubeconfig_valid

    is_kubeconfig_valid --> |yes| set_condition_secret_valid_to_true
    is_kubeconfig_valid --> |no| set_condition_secret_valid_to_false

    set_condition_secret_valid_to_true --> stop
    set_condition_secret_valid_to_false --> stop
```
