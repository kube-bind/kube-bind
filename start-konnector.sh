#!/bin/bash
set -ex

cd /home/rasel/go/src/go.kubeware.dev/kube-bind/
go install -v ./...
#./bin/konnector
IGNORE_GO_VERSION=1 make build
docker build --tag superm4n/konnector .
docker push superm4n/konnector
kubectl delete pod -n kube-bind --field-selector=metadata.namespace==kube-bind