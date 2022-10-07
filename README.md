<img alt="Logo" width="196px" style="margin-right: 30px;" align="left" src="./docs/images/logo.svg"></img>

# kube-bind

kube-bind is a prototype project with the goal to establish a new extension model for Kubernetes clusters:

- APIs should be bindable into a cluster and operated by a service provider
- these APIs should not require (custom) controllers/operators run locally in the consuming cluster
- only a single vendor-neutral, OpenSource agent should be required.

This is the 3 line pitch:

```shell
$ kubectl krew install bind
$ kubectl bind https://mangodb/exports
Redirect to the brower to authenticate via OIDC.
BOOM – the MangoDB API is available in the local cluster, 
       without anything MangoDB-specific running.
$ kubectl get mangodbs 
```

For more information go to https://kubectl-bind.io or watch the [ContainerDays talk](https://www.youtube.com/watch?v=dg0g15Qv5Fo&t=1s).

The kube-bind prototype is following this manifesto from the linked talk:

![kube-bind manifesto](docs/images/manifesto.png)
