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
OPENAPI_PKG=${OPENAPI_PKG:-$(cd "${SCRIPT_ROOT}"; go list -f '{{.Dir}}' -m k8s.io/kube-openapi)}

# TODO: This is hack to allow CI to pass
chmod +x "${CODEGEN_PKG}"/generate-internal-groups.sh

${KUBE_APPLYCONFIGURATION_GEN} \
  --go-header-file ./hack/boilerplate/boilerplate.generatego.txt \
  --output-pkg github.com/kube-bind/kube-bind/sdk/kcp/applyconfiguration \
  --output-dir "sdk/kcp/applyconfiguration" \
  github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha1 \
  github.com/kube-bind/kube-bind/sdk/apis/third_party/conditions/apis/conditions/v1alpha1 \
  k8s.io/apimachinery/pkg/apis/meta/v1 \
  k8s.io/apimachinery/pkg/runtime \
  k8s.io/apimachinery/pkg/version

"${KUBE_CLIENT_GEN}" \
  --input github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha1 \
  --input-base="" \
  --apply-configuration-package=github.com/kube-bind/kube-bind/sdk/kcp/applyconfiguration \
  --clientset-name "versioned"  \
  --output-pkg github.com/kube-bind/kube-bind/sdk/kcp/clientset \
  --go-header-file ./hack/boilerplate/boilerplate.generatego.txt \
  --output-dir "sdk/kcp/clientset"
pushd ./sdk/apis

${CODE_GENERATOR} \
  "client:outputPackagePath=github.com/kube-bind/kube-bind/sdk/kcp,apiPackagePath=github.com/kube-bind/kube-bind/sdk/apis,singleClusterClientPackagePath=github.com/kube-bind/kube-bind/sdk/kcp/clientset/versioned,singleClusterApplyConfigurationsPackagePath=github.com/kube-bind/kube-bind/sdk/kcp/applyconfiguration,headerFile=${BOILERPLATE_HEADER}" \
  "lister:apiPackagePath=github.com/kube-bind/kube-bind/sdk/apis,headerFile=${BOILERPLATE_HEADER}" \
  "informer:outputPackagePath=github.com/kube-bind/kube-bind/sdk/kcp,singleClusterClientPackagePath=github.com/kube-bind/kube-bind/sdk/kcp/clientset/versioned,apiPackagePath=github.com/kube-bind/kube-bind/sdk/apis,headerFile=${BOILERPLATE_HEADER}" \
  "paths=./..." \
  "output:dir=../kcp"
popd