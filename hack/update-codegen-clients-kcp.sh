#!/usr/bin/env bash

# Copyright 2021 The Kube Bind Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -o errexit
set -o nounset
set -o pipefail
set -o xtrace

export GOPATH=$(go env GOPATH)

SCRIPT_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
pushd "${SCRIPT_ROOT}"
BOILERPLATE_HEADER="$( pwd )/hack/boilerplate/boilerplate.generatego.txt"
popd
CODEGEN_PKG=${CODEGEN_PKG:-$(cd "${SCRIPT_ROOT}"; go list -f '{{.Dir}}' -m k8s.io/code-generator)}
CLUSTER_CODEGEN_PKG=${CLUSTER_CODEGEN_PKG:-$(cd "${SCRIPT_ROOT}"; go list -f '{{.Dir}}' -m github.com/kcp-dev/code-generator/v3)}
OPENAPI_PKG=${OPENAPI_PKG:-$(cd "${SCRIPT_ROOT}"; go list -f '{{.Dir}}' -m k8s.io/kube-openapi)}

go install "${CODEGEN_PKG}"/cmd/applyconfiguration-gen
go install "${CODEGEN_PKG}"/cmd/client-gen

# TODO: This is hack to allow CI to pass
chmod +x "${CODEGEN_PKG}"/generate-internal-groups.sh

source "${CODEGEN_PKG}/kube_codegen.sh"
source "${CLUSTER_CODEGEN_PKG}/cluster_codegen.sh"

rm -rf ${SCRIPT_ROOT}/sdk/kcp/{clientset,applyconfiguration,listers,informers}
mkdir -p ${SCRIPT_ROOT}/sdk/kcp/{clientset,applyconfiguration,listers,informers}

"$GOPATH"/bin/applyconfiguration-gen \
  --go-header-file ./hack/boilerplate/boilerplate.generatego.txt \
  --output-pkg github.com/kube-bind/kube-bind/sdk/kcp/applyconfiguration \
  --output-dir "sdk/kcp/applyconfiguration" \
  github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha1 \
  github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2 \
  github.com/kube-bind/kube-bind/sdk/apis/third_party/conditions/apis/conditions/v1alpha1 \
  k8s.io/apimachinery/pkg/apis/meta/v1 \
  k8s.io/apimachinery/pkg/runtime \
  k8s.io/apimachinery/pkg/version

"$GOPATH"/bin/client-gen \
  --input github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha1 \
  --input github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2 \
  --input-base="" \
  --apply-configuration-package=github.com/kube-bind/kube-bind/sdk/kcp/applyconfiguration \
  --clientset-name "versioned"  \
  --output-pkg github.com/kube-bind/kube-bind/sdk/kcp/clientset \
  --go-header-file ./hack/boilerplate/boilerplate.generatego.txt \
  --output-dir "sdk/kcp/clientset"


# Install cluster codegen tools
# HACK: for some reason using bash wrapper does not work due to go mod structure.
# we need to install them directly. If you read this - feel free to fix it.
go install github.com/kcp-dev/code-generator/v3/cmd/cluster-client-gen
go install github.com/kcp-dev/code-generator/v3/cmd/cluster-informer-gen
go install github.com/kcp-dev/code-generator/v3/cmd/cluster-lister-gen

GOBIN=$(go env GOPATH)/bin

# Generate cluster client
${GOBIN}/cluster-client-gen \
  -v 0 \
  --go-header-file "${BOILERPLATE_HEADER}" \
  --output-dir "sdk/kcp/clientset/versioned" \
  --output-pkg github.com/kube-bind/kube-bind/sdk/kcp/clientset/versioned \
  --input-base "" \
  --single-cluster-versioned-clientset-pkg github.com/kube-bind/kube-bind/sdk/kcp/clientset/versioned \
  --single-cluster-applyconfigurations-pkg github.com/kube-bind/kube-bind/sdk/kcp/applyconfiguration \
  --input github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha1 \
  --input github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2

# Generate cluster listers
${GOBIN}/cluster-lister-gen \
  -v 0 \
  --go-header-file "${BOILERPLATE_HEADER}" \
  --output-dir "sdk/kcp/listers" \
  --output-pkg github.com/kube-bind/kube-bind/sdk/kcp/listers \
  github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha1 \
  github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2

# Generate cluster informers  
${GOBIN}/cluster-informer-gen \
  -v 0 \
  --go-header-file "${BOILERPLATE_HEADER}" \
  --output-dir "sdk/kcp/informers/externalversions" \
  --output-pkg github.com/kube-bind/kube-bind/sdk/kcp/informers/externalversions \
  --versioned-clientset-pkg github.com/kube-bind/kube-bind/sdk/kcp/clientset/versioned \
  --listers-pkg github.com/kube-bind/kube-bind/sdk/kcp/listers \
  --single-cluster-versioned-clientset-pkg github.com/kube-bind/kube-bind/sdk/kcp/clientset/versioned \
  github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha1 \
  github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2