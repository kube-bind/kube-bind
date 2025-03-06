# APIServiceExports

The APIServiceExport controller watches `APIServiceExports` and the referenced `CustomResourceDefinitions` (CRDs) in the **provider cluster**.

It is responsible for:

* Ensuring the existence and validity of `APIServiceExports`.
* Managing the lifecycle of `APIServiceExports` by starting and stopping spec and status syncers and controllers.
* Checking `APIServiceBinding` condition and setting `ConsumerInSync` condition in `APIServiceExport`
* Copying conditions from the referenced CRDs to the `APIServiceExport` status.

## Overview

```mermaid
flowchart TD
    %% Nodes
    start@{ shape: start }
    stop@{ shape: stop }

    enqueue_reconcile(["Enqueue reconcile call
    for APIServiceExport"])
    is_apiserviceexport_present{"APIServiceExport<br>exists?"}

    get_crd(["Get referenced
    CustomResourceDefinition"])
    get_crd2(["Get referenced
    CustomResourceDefinition"])

    is_crd_present{"CRD<br>exists?"}
    is_crd_present2{"CRD<br>exists?"}

    stop_apiserviceexport_sync(["Stop APIServiceExport sync"])

    get_binding(["Get referenced
    APIServiceBinding"])
    get_binding2(["Get referenced APIServiceBinding"])

    is_binding_present{"APIServiceBinding<br>exists?"}
    is_binding_present2{"APIServiceBinding<br>exists?"}

    is_binding_schemainsync{"APIServiceBinding<br>has SchemaInSync?"}
    start_new_syncer(["Start new syncer"])

    set_apiserviceexport_condition_connected_to_true(["Set condition
    'Connected' to true"])
    set_apiserviceexport_condition_connected_to_false(["Set
    condition 'Connected'
to false"])
    set_apiserviceexport_condition_consumerinsync_to_true(["Set condition
    'ConsumerInSync' to true"])
    set_apiserviceexport_condition_consumerinsync_to_false(["Set condition
    'ConsumerInSync' to false"])

    copy_crd_conditions(["Copy CRD conditions
    to APIServiceExport
    conditions"])
    set_summary_condition(["Set summary
    of all conditions
    on APIServiceExport"])


    %% Transitions
    start --> enqueue_reconcile
    enqueue_reconcile --> is_apiserviceexport_present
    is_apiserviceexport_present --> |no| stop
    is_apiserviceexport_present --> |yes| get_crd
    get_crd --> is_crd_present
    is_crd_present --> |yes| get_binding
    is_crd_present --> |no| stop_apiserviceexport_sync
    stop_apiserviceexport_sync --> stop

    get_binding --> is_binding_present
    is_binding_present --> |yes| start_new_syncer
    is_binding_present --> |no| stop_apiserviceexport_sync
    stop_apiserviceexport_sync --> stop

    start_new_syncer --> get_binding2
    get_binding2 --> is_binding_present2
    is_binding_present2 --> |yes| set_apiserviceexport_condition_connected_to_true
    is_binding_present2 --> |no| set_apiserviceexport_condition_connected_to_false
    set_apiserviceexport_condition_connected_to_false --> set_apiserviceexport_condition_consumerinsync_to_false
    set_apiserviceexport_condition_connected_to_true --> is_binding_schemainsync
    is_binding_schemainsync --> |yes| set_apiserviceexport_condition_consumerinsync_to_true
    set_apiserviceexport_condition_consumerinsync_to_true --> get_crd2
    is_binding_schemainsync --> |no| set_apiserviceexport_condition_consumerinsync_to_false
    set_apiserviceexport_condition_consumerinsync_to_false --> get_crd2

    get_crd2 --> is_crd_present2
    is_crd_present2 --> |no| stop
    is_crd_present2 --> |yes| copy_crd_conditions
    copy_crd_conditions --> set_summary_condition
    set_summary_condition --> stop
```
