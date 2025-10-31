# Migration Guide

This guide helps you migrate from older versions of kube-bind to the latest version with new features and API changes.

## Migration Timeline

### From v0.5.x to v0.6.x+

The v0.6.x release introduces several significant improvements:

- **Catalog API**: New `Collection` and `APIServiceExportTemplate` CRDs for better service organization
- **Enhanced Permission Claims**: Support for `NamedResources` alongside label selectors
- **Provider-side Namespace Management**: Automatic RBAC and namespace provisioning
- **Improved kcp Integration**: Better workspace and APIExport handling

## API Changes

TODO

If you encounter issues during migration:

1. **Check GitHub Issues**: [kube-bind issues](https://github.com/kube-bind/kube-bind/issues)
2. **Slack Channel**: [`#kube-bind` on Kubernetes Slack](https://kubernetes.slack.com/archives/C046PRXNJ4W)
3. **Mailing List**: [kube-bind-dev](https://groups.google.com/g/kube-bind-dev)
