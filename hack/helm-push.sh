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

echo "Pushing Helm charts to registry: $IMAGE_REPO"

HELM="$(UGET_PRINT_PATH=absolute make --no-print-directory install-helm)"
CHART_VERSION="0.0.0-$REV"

export HELM_EXPERIMENTAL_OCI=1

for chart_file in ./bin/*-$CHART_VERSION.tgz; do
   if [ -f "$chart_file" ]; then
      chart_filename=$(basename "$chart_file")
      chart_name=${chart_filename%-$CHART_VERSION.tgz}

      if [[ "$chart_name" =~ [[:space:]] ]]; then
         echo "Skipping chart with invalid name: '$chart_name' (contains spaces)"
         continue
      fi

      echo "Pushing $chart_name to $(IMAGE_REPO)"
      "$HELM" push "$chart_file" "oci://$(IMAGE_REPO)/charts"
      echo "Chart available at: oci://$(IMAGE_REPO)/charts/$chart_name:$CHART_VERSION"
   fi
done
