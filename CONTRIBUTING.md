# Contributing to kube-bind

kube-bind is [Apache 2.0 licensed](LICENSE) and we accept contributions via
GitHub pull requests.

Please read the following guide if you're interested in contributing to kube-bind.

## Certificate of Origin

By contributing to this project you agree to the Developer Certificate of
Origin (DCO). This document was created by the Linux Kernel community and is a
simple statement that you, as a contributor, have the legal right to make the
contribution. See the [DCO](DCO) file for details.

## Getting started

### Prerequisites

1. Clone this repository.
2. [Install Go](https://golang.org/doc/install) (1.18+).
3. Install [kubectl](https://kubernetes.io/docs/tasks/tools/#kubectl).

### Build & verify
1. In one terminal, build and start `bin/kubectl-bind`:
```
make
bin/kubectl-bind
```

## Finding areas to contribute

Starting to participate in a new project can sometimes be overwhelming, and you may not know where to begin. Fortunately, we are here to help! We track all of our tasks here in GitHub, and we label our issues to categorize them. Here are a couple of handy links to check out:

* [Good first issue](https://github.com/kube-bind/kube-bind/issues?q=is%3Aopen+is%3Aissue+label%3A%22good+first+issue%22) issues
* [Help wanted](https://github.com/kube-bind/kube-bind/issues?q=is%3Aopen+is%3Aissue+label%3A%22help+wanted%22) issues

You're certainly not limited to only these kinds of issues, though! If you're comfortable, please feel free to try working on anything that is open.

We do use the assignee feature in GitHub for issues. If you find an unassigned issue, comment asking if you can be assigned, and ideally wait for a maintainer to respond. If you find an assigned issue and you want to work on it or help out, please reach out to the assignee first.

Sometimes you might get an amazing idea and start working on a huge amount of code. We love and encourage excitement like this, but we do ask that before you embarking on a giant pull request, please reach out to the community first for an initial discussion. You could [file an issue](https://github.com/kube-bind/kube-bind/issues/new/choose).

Finally, we welcome and value all types of contributions, beyond "just code"! Other types include triaging bugs, tracking down and fixing flaky tests, improving our documentation, helping answer community questions, proposing and reviewing designs, etc.

## Coding guidelines & conventions

- Logging:
  - We use klog's contextual logging: `logging := klog.FromContext(ctx)`.
  - Default log-level is 2.
  - Controllers should generally log (a) **one** line (not more) non-error progress per item with `klog.Info` (b) actions like create/update/delete via `klog.V(1).Info` and (c) skipped actions, i.e. what was not done for reasons via `klog.V(2).Info`.
- Go Proverbs are good guidelines for style: https://go-proverbs.github.io/ – watch https://www.youtube.com/watch?v=PAAkCSZUG1c.
- We use [https://github.com/stretchr/testify/tree/master/require](github.com/stretchr/testify/require) a lot in tests, and avoid
  [https://github.com/stretchr/testify/tree/master/assert](https://github.com/stretchr/testify/assert).

  Note this subtle distinction of nested `require` statements:
  ```Golang
  require.Eventually(t, func() bool {
    foos, err := client.List(...)
    require.NoError(err) // fail fast, including failing require.Eventually immediately
    return someCondition(foos)
  }, ...)
  ```
  and
  ```Golang
  require.Eventually(t, func() bool {
    foos, err := client.List(...)
    if err != nil {
       return false // keep trying
    }
    return someCondition(foos)
  }, ...)
  ```
  The first fails fast on every client error. The second ignores client errors and keeps trying. Either
  has its place, depending on whether the client error is to be expected (e.g. because of asynchronicity making the resource available),
  or signals a real test problem.

### Using Kubebuilder CRD Validation Annotations

All of the API resources for `kube-bind` are `CustomResourceDefinitions`, and we generate YAML spec for them from our Go types using [kubebuilder](https://github.com/kubernetes-sigs/kubebuilder).

When adding a field that requires validation, custom annotations are used to translate this logic into the generated OpenAPI spec. [This doc](https://book.kubebuilder.io/reference/markers/crd-validation.html) gives an overview of possible validations. These annotations map directly to concepts in the [OpenAPI Spec](https://swagger.io/specification/#data-type-format) so, for instance, the `format` of strings is defined there, not in kubebuilder. Furthermore, Kubernetes has forked the OpenAPI project [here](https://github.com/kubernetes/kube-openapi/tree/master/pkg/validation) and extends more formats in the extensions-apiserver [here](https://github.com/kubernetes/kubernetes/blob/master/staging/src/k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1/types_jsonschema.go#L27).

## Community Roles

### Reviewers

Reviewers are responsible for reviewing code for correctness and adherence to standards. Oftentimes reviewers will
be able to advise on code efficiency and style as it relates to golang or project conventions as well as other considerations
that might not be obvious to the contributor.

### Approvers

Approvers are responsible for sign-off on the acceptance of the contribution. In essence, approval indicates that the
change is desired and good for the project, aligns with code, api, and system conventions, and appears to follow all required
process including adequate testing, documentation, follow ups, or notifications to other areas who might be interested
or affected by the change.

Approvers are also reviewers.
