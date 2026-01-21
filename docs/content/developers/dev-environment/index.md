---
description: >
  How to setup a development environment for contributing to kube-bind.
title: Development Environments
---

# Development Environments

Due to the fact that kube-bind is by nature a multi-cluster system, for development purposes it's recommended to have multiple clusters running or use kcp to simulate multiple clusters. Below are instructions for both approaches.

All the instructions assume you have already cloned the kube-bind repository and have Go installed.

* You can use [kcp](kcp.md) for a lightweight backend system.
* You can also use [kind](kind.md) for a more full-featured local Kubernetes cluster.
