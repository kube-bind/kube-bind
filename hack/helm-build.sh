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

# Inputs (env vars):
#   REV             short git sha (required if CHART_VERSION is unset)
#   CHART_VERSION   chart version string; default: 0.0.0-$REV

: "${CHART_VERSION:=0.0.0-${REV:?REV must be set when CHART_VERSION is unset}}"

echo "Building Helm charts (version: $CHART_VERSION)..."

HELM="$(UGET_PRINT_PATH=absolute make --no-print-directory install-helm)"

for chart_dir in deploy/charts/*/; do
   if [ -f "${chart_dir}Chart.yaml" ]; then
      chart_name=$(basename "$chart_dir")
      echo "Processing chart: $chart_name"

      cp "${chart_dir}Chart.yaml" "${chart_dir}Chart.yaml.bak"
      sed -i.tmp "s/^version:.*/version: $CHART_VERSION/" "${chart_dir}Chart.yaml"
      sed -i.tmp "s/^appVersion:.*/appVersion: v$CHART_VERSION/" "${chart_dir}Chart.yaml"
      rm -f "${chart_dir}Chart.yaml.tmp"

      "$HELM" package "$chart_dir" --version "$CHART_VERSION" --destination ./bin/
      echo "Packaged: ./bin/$chart_name-$CHART_VERSION.tgz"

      mv "${chart_dir}Chart.yaml.bak" "${chart_dir}Chart.yaml"
   fi
done

echo "Helm charts built successfully in ./bin/"
