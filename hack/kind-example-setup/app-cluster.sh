#!/usr/bin/env bash

# Copyright 2025 The Kube Bind Authors.
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

# This script ensures that the generated client code checked into git is up-to-date
# with the generator. If it is not, re-generate the configuration to update it.


set -o nounset
set -o pipefail

export APP_HOST_IP=192.168.0.34
export BACKEND_HOST_IP=192.168.0.34

cat << EOF_AppClusterDefinition | kind create cluster --config=-
apiVersion: kind.x-k8s.io/v1alpha4
kind: Cluster
name: app
networking:
  apiServerAddress: ${APP_HOST_IP}
EOF_AppClusterDefinition

kubectl-bind bind http://${BACKEND_HOST_IP}:8080/export
