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

set -eu

echo "Testing Helm chart installation..."

HELM="$(UGET_PRINT_PATH=absolute make --no-print-directory install-helm)"
CHART_VERSION="0.0.0-$REV"

for chart_dir in deploy/charts/*/; do
   if [ -f "${chart_dir}Chart.yaml" ]; then
      chart_name=$(basename "$chart_dir")
      echo "Testing chart: $chart_name"
      "$HELM" install test-$chart_name "./bin/$chart_name-$CHART_VERSION.tgz" --dry-run --debug
      echo "✓ Chart $chart_name passes dry-run test"
   fi
done
