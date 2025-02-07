# API Binding

## Overview

```mermaid
sequenceDiagram
    autonumber
    participant consumer-cluster as Consumer cluster
    participant client as Client
    participant provider-cluster as Provider cluster
    participant provider-backend as Provider backend


    %% Create APIServiceExportRequest
    client->>provider-cluster: Create "APIServiceExportRequest"

    par Provider
      provider-backend->>provider-cluster: Get "APIServiceExportRequest"
      loop For each "APIServiceExportRequest.spec.resources"
        provider-backend->>provider-cluster: Create "APIServiceExport"
      end
      provider-backend->>provider-cluster: Set "APIServiceExportRequest.status.phase"
    and Client
      loop Every 1 second for 10 minutes
      client->>provider-cluster: Get "APIServiceExportRequest"
      client->>client: Verify "APIServiceExportRequest"<br/>(.status.phase == Succeeded)"
      end
    end

    %% Create APIServiceBindings
    loop For each "APIServiceExportRequest.spec.resources"
    client->>consumer-cluster: Create "APIServiceBinding"
    end
```

## Contracts

**APIServiceExportRequest**

In case the _APIServiceExportRequest_ is accepted, the provider backend **must** ensure that

* for each `APIServiceExportRequest.spec.resources` an `APIServiceExport` is created
* each `APIServiceExport` is created in the namespace of the `APIServiceExportRequest`
* each `APIServiceExport` is created with a name following the pattern `resource.resource + "." + resource.group`
* the `APIServiceExportRequest.status.phase` is set to `Succeeded`

In case the _APIServiceExportRequest_ is declined, the provider backend **must** ensure that

* the `APIServiceExportRequest.status.terminalMessage` is set to a human readable message describing the reason
* the `APIServiceExportRequest.status.phase` is set to `Failed`
