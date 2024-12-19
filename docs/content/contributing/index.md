# Contributing to kube-bind

kube-bind is [Apache 2.0 licensed](https://github.com/kube-bind/kube-bind/tree/main/LICENSE) and we accept contributions via
GitHub pull requests.

Please read the following guide if you're interested in contributing to kcp.

## Certificate of Origin

By contributing to this project you agree to the Developer Certificate of
Origin (DCO). This document was created by the Linux Kernel community and is a
simple statement that you, as a contributor, have the legal right to make the
contribution. See the [DCO](https://github.com/kube-bind/kube-bind/tree/main/DCO) file for details.

## Getting Started

### Prerequisites

1. Clone this repository.
2. [Install Go](https://golang.org/doc/install) (currently 1.22).
3. Install [kubectl](https://kubernetes.io/docs/tasks/tools/#kubectl).

## Finding Areas to Contribute

Starting to participate in a new project can sometimes be overwhelming, and you may not know where to begin. Fortunately, we are here to help! We track all of our tasks here in GitHub, and we label our issues to categorize them. Here are a couple of handy links to check out:

* [Good first issue](https://github.com/kcp-dev/kcp/issues?q=is%3Aopen+is%3Aissue+label%3A%22good+first+issue%22) issues
* [Help wanted](https://github.com/kcp-dev/kcp/issues?q=is%3Aopen+is%3Aissue+label%3A%22help+wanted%22) issues

You're certainly not limited to only these kinds of issues, though! If you're comfortable, please feel free to try working on anything that is open.

We do use the assignee feature in GitHub for issues. If you find an unassigned issue, comment asking if you can be assigned, and ideally wait for a maintainer to respond. If you find an assigned issue and you want to work on it or help out, please reach out to the assignee first.

Sometimes you might get an amazing idea and start working on a huge amount of code. We love and encourage excitement like this, but we do ask that before you embarking on a giant pull request, please reach out to the community first for an initial discussion. You could [file an issue](https://github.com/kube-bind/kube-bind/issues/new/choose), send a discussion to our [mailing list](https://groups.google.com/g/kube-bind-dev), and/or join one of our [community meetings](https://docs.google.com/document/d/1PrEhbmq1WfxFv1fTikDBZzXEIJkUWVHdqDFxaY1Ply4).

Finally, we welcome and value all types of contributions, beyond "just code"! Other types include triaging bugs, tracking down and fixing flaky tests, improving our documentation, helping answer community questions, proposing and reviewing designs, etc.

### Getting your PR Merged

The `kube-bind` project uses `OWNERS` files to denote the collaborators who can assist you in getting your PR merged.  There
are two roles: reviewer and approver.  Merging a PR requires sign off from both a reviewer and an approver.

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

### Management of `OWNERS` Files

If a reviewer or approver no longer wishes to be in their current role it is requested that a PR
be opened to update the `OWNERS` file. `OWNERS` files may be periodically reviewed and updated based on project activity
or feedback to ensure an acceptable contributor experience is maintained.
