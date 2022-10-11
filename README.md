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

## Technical Overview

<img alt="overview" width="800px" src="./docs/images/overview.png"></img>

## Usage

To run the current backend, there must be an OIDC issuer installed in place to do the
the oauth2 workflow.

We use dex to manage OIDC, following the steps below you can run a local OIDC issuer using dex:
* First, clone the dex repo: `git clone https://github.com/dexidp/dex.git`
* `cd dex` and then build the dex binary `make build`
* The binary will be created in `bin/dex`
* Adjust the config file(`examples/config-dev.yaml`) for dex by specifying the server callback method:
```yaml
staticClients:
- id: kube-bind
  redirectURIs:
  - 'http://127.0.0.1:8080/callback'
  name: 'Kube Bind'
```
* Run dex: `./bin/dex serve examples/config-dev.yaml`

Next you should be able to run the backend. For it you need a kubernetes cluster (e.g. kind)
accessible.

***Note: make sure before running the backend that you have the dex server up and running as mentioned above
and that you have at least one k8s cluster. Take a look at the backend option in the cmd/main.go file***

* apply the CRDs: `kubectl apply -f deploy/crd`
* start the backend binary with the right flags:
```shell
$ make build
$ bin/example-backend \
  --namespace=[whatever-namespace|default kube-system] \
  --oidc-issuer-client-secret=ZXhhbXBsZS1hcHAtc2VjcmV0 \
  --oidc-issuer-client-id=kube-bind \
  --oidc-issuer-url=http://127.0.0.1:5556/dex \
  --cluster-name=[k8s-clustername]
```
where `ZXhhbXBsZS1hcHAtc2VjcmV0` matches the value of the dex config file.

* browse to `http://127.0.0.1:8080/authorize` in a browser.
* login with the "Example" connector.
* you are redirected by to the backend.