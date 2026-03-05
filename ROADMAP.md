# kube-bind Roadmap

This roadmap reflects the current direction of the kube-bind project.
Items are grouped by theme, not strict release milestones.
Track progress via [GitHub Issues](https://github.com/kube-bind/kube-bind/issues).

## 🏗️ Production Readiness

- **Pluggable session storage** — replace in-memory backend session storage with
  an external/pluggable implementation, enabling replicas > 1 for HA deployments
  ([#424](https://github.com/kube-bind/kube-bind/issues/424),
  [#488](https://github.com/kube-bind/kube-bind/issues/488))
- **CRD lifecycle / BoundSchema changes** — support schema evolution after initial
  binding, including updates and versioning
  ([#404](https://github.com/kube-bind/kube-bind/issues/404))
- **Delete flow** — full lifecycle support for APIServiceExport deletion and
  graceful unbinding
  ([#352](https://github.com/kube-bind/kube-bind/issues/352))
- **Backend production hardening** — address remaining gaps for production-grade
  deployments ([#284](https://github.com/kube-bind/kube-bind/issues/284))

## 🔌 Feature Completeness

- **Related resources** — support binding of associated resources such as
  ConfigMaps and Secrets alongside primary CRDs
- **Multi-provider/consumer support** — allow a single consumer cluster to bind
  the same CRD from multiple providers, and other multi-tenancy scenarios
  ([#321](https://github.com/kube-bind/kube-bind/issues/321))
- **Object metadata propagation** — propagate selected labels/annotations between
  provider and consumer objects
  ([#315](https://github.com/kube-bind/kube-bind/issues/315))
- **Reduced konnector permissions** — tighten RBAC footprint of the konnector
  agent in consumer clusters
  ([#303](https://github.com/kube-bind/kube-bind/issues/303))
- **UI-only binding flow** — allow service binding without CLI, purely via
  browser/OIDC redirect flow
  ([#406](https://github.com/kube-bind/kube-bind/issues/406))
- **Dry-run support** — fix `--dry-run` in `bind apiservice`
  ([#365](https://github.com/kube-bind/kube-bind/issues/365))

## 🌐 Ecosystem Integration

- **kcp integration** — make the konnector kcp-aware and support serving from
  APIBindings ([#317](https://github.com/kube-bind/kube-bind/issues/317),
  [#300](https://github.com/kube-bind/kube-bind/issues/300),
  [#323](https://github.com/kube-bind/kube-bind/issues/323))
- **multicluster-runtime refactor** — refactor the konnector to use
  [multicluster-runtime](https://github.com/kubernetes-sigs/multicluster-runtime)
  for improved multi-cluster controller patterns
  ([#299](https://github.com/kube-bind/kube-bind/issues/299))
- **PlatformMesh integration** — provider-side integration with PlatformMesh
  ([#447](https://github.com/kube-bind/kube-bind/issues/447))

## 📖 Documentation & Community

- Blog post on external integrations
  ([#452](https://github.com/kube-bind/kube-bind/issues/452))
- CNCF Sandbox application
  ([#448](https://github.com/kube-bind/kube-bind/issues/448))
- Adopters file

---

This roadmap is maintained by the [kube-bind maintainers](./MAINTAINERS.md).
Contributions and feedback welcome via
[GitHub Discussions](https://github.com/kube-bind/kube-bind/discussions) or
[`#kube-bind`](https://kubernetes.slack.com/archives/C046PRXNJ4W) on Kubernetes Slack.
