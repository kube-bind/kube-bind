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
CODEGEN_PKG=${CODEGEN_PKG:-$(cd "${SCRIPT_ROOT}"; go list -f '{{.Dir}}' -m k8s.io/code-generator)}

source "${CODEGEN_PKG}/kube_codegen.sh"

pushd ./sdk/apis
kube::codegen::gen_helpers \
    --boilerplate "../../hack/boilerplate/boilerplate.generatego.txt" \
    "."

kube::codegen::gen_client \
    --with-watch \
    --output-dir "../client" \
    --output-pkg "github.com/kube-bind/kube-bind/sdk/client" \
    --boilerplate "../../hack/boilerplate/boilerplate.generatego.txt" \
    "."

popd

