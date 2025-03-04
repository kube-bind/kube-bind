# APIServiceBindings

The APIServiceBinding controller watches `APIServiceBindings`, and `CRDs` in the **consumer cluster** and `APIServiceExports` in the **provider cluster**.

It is responsible for:

* synchronizing `APIServiceExports` in the **provider cluster** to `CRDs` in the **consumer cluster**

## Overview

```mermaid
flowchart TD
    %% Nodes
    start@{ shape: start }
    stop@{ shape: stop }

    enqueue_reconcile(["Enqueue reconcile call for APIServiceBinding"])

    is_apiservicebinding_owned{"binding<br>owned?"}
    set_apiservicebinding_condition_connected_to_true(["Set condition 'Connected' to true"])
    set_apiservicebinding_condition_connected_to_false(["Set condition 'Connected' to false"])
    set_apiservicebinding_condition_schemainsync_to_true(["Set condition 'SchemaInSync' to true"])
    set_apiservicebinding_condition_schemainsync_to_false(["Set condition 'SchemaInSync' to false"])

    get_apiserviceexport(["Get APIServiceExport"])
    convert_apiserviceexport_to_crd(["Convert APIServiceExport to CRD"])
    is_apiserviceexport_present{"export<br>exists?"}
    is_apiserviceexport_valid{"export<br>valid?"}

    get_crd(["Get CRD"])
    create_crd(["Create CRD"])
    update_crd(["Update CRD"])
    is_crd_present{"CRD<br>exists?"}
    is_crd_owned{"CRD<br>owned?"}

    get_clusterbinding(["Get ClusterBinding"])
    set_apiservicebinding_provider_name(["Set provider name"])

    %% Transitions
    start --> enqueue_reconcile
    enqueue_reconcile --> is_apiservicebinding_owned

    is_apiservicebinding_owned --> |yes| get_apiserviceexport
    get_apiserviceexport --> is_apiserviceexport_present
    is_apiserviceexport_present --> |yes| set_apiservicebinding_condition_connected_to_true
    is_apiserviceexport_present --> |no| set_apiservicebinding_condition_connected_to_false
    set_apiservicebinding_condition_connected_to_true --> convert_apiserviceexport_to_crd
    set_apiservicebinding_condition_connected_to_false --> stop

    convert_apiserviceexport_to_crd --> is_apiserviceexport_valid
    is_apiserviceexport_valid --> |yes| get_crd
    is_apiserviceexport_valid --> |no| set_apiservicebinding_condition_schemainsync_to_false

    get_crd --> is_crd_present
    is_crd_present --> |no| create_crd
    is_crd_present --> |yes| is_crd_owned
    update_crd --> set_apiservicebinding_condition_schemainsync_to_true
    create_crd --> set_apiservicebinding_condition_schemainsync_to_true

    is_crd_owned --> |yes| update_crd
    is_crd_owned --> |no| set_apiservicebinding_condition_schemainsync_to_false

    set_apiservicebinding_condition_schemainsync_to_true --> get_clusterbinding
    set_apiservicebinding_condition_schemainsync_to_false --> get_clusterbinding

    get_clusterbinding --> set_apiservicebinding_provider_name

    is_apiservicebinding_owned --> |no| stop
    set_apiservicebinding_provider_name --> stop
```
