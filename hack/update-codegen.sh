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

if [[ -z "${MAKELEVEL:-}" ]]; then
    echo 'You must invoke this script via make'
    exit 1
fi

"$(dirname "${BASH_SOURCE[0]}")/update-codegen-clients.sh"

# Update generated CRD YAML
cd pkg/apis
../../${CONTROLLER_GEN} \
    crd \
    rbac:roleName=manager-role \
    webhook \
    paths="./..." \
    output:crd:artifacts:config=../../deploy/crd
cd -

# Update generated CRD YAML
cd contrib/example-backend/apis
../../../${CONTROLLER_GEN} \
    crd \
    rbac:roleName=manager-role \
    webhook \
    paths="./..." \
    output:crd:artifacts:config=../../../contrib/deploy/crd
cd -


cd deploy/crd
for CRD in *.yaml; do
    if [ -f "../patches/${CRD}-patch" ]; then
        echo "Applying ${CRD}-patch"
        ../../${YAML_PATCH} -o "../patches/${CRD}-patch" < "${CRD}" > "${CRD}.patched"
        mv "${CRD}.patched" "${CRD}"
    fi
done
