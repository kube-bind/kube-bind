---
description: >
  How to install and use the kubectl bind plugin.
---

# kubectl bind Plugin

The `kubectl bind` plugin is the primary command-line interface for interacting with kube-bind services. It provides both interactive web UI access and command-line binding capabilities for connecting to remote service providers.

## Installation

=== "Krew"

    Install the plugin using [krew](https://krew.sigs.k8s.io/):

    ```bash
    kubectl krew index add bind https://github.com/kube-bind/krew-index.git
    kubectl krew install bind/bind
    ```

=== "Manual Build"

    Build and install from source:

    ```bash
    git clone https://github.com/kube-bind/kube-bind.git
    cd kube-bind
    make build
    cp bin/kubectl-bind /usr/local/bin/
    ```

=== "Binary Download"

    Download pre-built binaries from the [releases page](https://github.com/kube-bind/kube-bind/releases):

    ```bash
    # Download and install for Linux/macOS
    curl -LO https://github.com/kube-bind/kube-bind/releases/latest/download/kubectl-bind-linux-amd64
    chmod +x kubectl-bind-linux-amd64
    sudo mv kubectl-bind-linux-amd64 /usr/local/bin/kubectl-bind
    ```

## Basic Usage

The main plugin command is `kubectl bind` which opens the kube-bind web UI in your browser for interactive service binding.

```bash
# Login to a kube-bind server first
kubectl bind login https://my-kube-bind-server.example.com

# Open kube-bind UI for current server context
kubectl bind
```

The plugin provides several subcommands including `login`, `templates`, `collections`, and `apiservice` for different binding workflows.

For complete command reference and examples, see the [CLI Reference](../reference/index.md).

## Quick Start

1. Install the plugin using Krew or build from source
2. Login to your kube-bind server: `kubectl bind login <server-url>`
3. Open the web UI: `kubectl bind`
4. Browse and bind to available services through the interface

For detailed setup instructions, see the [Quickstart Guide](./quickstart.md).

