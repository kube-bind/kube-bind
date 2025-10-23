<img alt="Logo" width="196px" style="margin-right: 30px;" align="left" src="./docs/images/logo.svg"></img>

# kube-bind

### Disclaimer: work in progress and not ready for production use.

You are invited to [contribute](#contributing)!

## What is it?

kube-bind is a prototype project that aims to provide better support for service providers and consumers that reside in distinct Kubernetes clusters.

- A service provider defines its API in terms of CRDs and associated permission claims/limitations, and exports it for use from other clusters.
- Service consumers identify the services they want to consume.
- The service CRDs get installed in the service consumer clusters, with objects of the defined kinds written and read by the service consumers.
- The service provider indirectly reads and writes those objects as the interface to the service that it provides.
- The service provider does not inject controllers/operators into the service consumer's cluster.
- A single vendor-neutral, OpenSource agent per consumer cluster connects it with the requested services.

## Try it out

This is the 3 line pitch:

```shell
$ kubectl krew index add bind https://github.com/kube-bind/krew-index.git
$ kubectl krew install bind/bind
$ kubectl bind https://mangodb/exports
Redirect to the brower to authenticate via OIDC.
BOOM – the MangoDB API is available in the local cluster,
       without anything MangoDB-specific running.
$ kubectl get mangodbs
```

## For more information

For more information go to https://kube-bind.io or watch the [ContainerDays talk](https://www.youtube.com/watch?v=dg0g15Qv5Fo&t=1s)
or the [KubeCon talk](https://www.youtube.com/watch?v=Uv0ivz5xej4).

The kube-bind prototype is following this manifesto from the linked talk:

![kube-bind manifesto](docs/images/manifesto.png)

## Contributing

We ❤️ our contributors! If you're interested in helping us out, please check out
[Contributing to kube-bind](./CONTRIBUTING.md) and [kube-bind Project Governance](./GOVERNANCE.md).

## Getting in touch

There are several ways to communicate with us:

- The [`#kube-bind` channel](https://kubernetes.slack.com/archives/C046PRXNJ4W) in the [Kubernetes Slack workspace](https://slack.k8s.io)
- Our mailing list [kube-bind-dev](https://groups.google.com/g/kube-bind-dev) for development discussions.

## Technical Overview

<img alt="overview" width="800px" src="./docs/images/overview.png"></img>

All the actions shown between the clusters are done by the konnector, except: the pull at the start is done by the kubectl plugin that installs the konnector.

## Usage

To get familiar with setting up the environment, please check out docs at [kube-bind.io](https://docs.kube-bind.io/main/setup).

### Limitations

These limitations are part of the roadmap and will be addressed in the future.

* Currently we don't support related resources, like ConfigMaps, Secrets.
* We don't support lifecycle of `BoundSchema` resources, like schema changes.
* Currently multicluster-runtime dependency is pointing to a fork, we will work on getting the changes merged upstream. See [PR](https://github.com/kubernetes-sigs/multicluster-runtime/pull/62) for details.