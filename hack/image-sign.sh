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
#   IMAGE_REPO     registry+namespace, e.g. ghcr.io/kube-bind (required)
#   OUTPUT_DIR     directory containing <component>.metadata.json files (required)
#   COMPONENTS     space-separated component names; default: "konnector backend"
#   COSIGN_ARGS    extra args for `cosign sign` (e.g. -a sha=... -a ref=...)

: "${IMAGE_REPO:?IMAGE_REPO must be set}"
: "${OUTPUT_DIR:?OUTPUT_DIR must be set (directory with image build metadata files)}"
: "${COMPONENTS:=konnector backend}"

if ! command -v cosign >/dev/null 2>&1; then
   echo "cosign not found in PATH; skipping image signing."
   echo "  install: https://docs.sigstore.dev/cosign/installation/"
   exit 0
fi

if ! command -v jq >/dev/null 2>&1; then
   echo "jq not found in PATH; required to parse buildx metadata." >&2
   exit 1
fi

export COSIGN_EXPERIMENTAL=true

for component in $COMPONENTS; do
   metadata_file="${OUTPUT_DIR}/${component}.metadata.json"
   if [[ ! -f "$metadata_file" ]]; then
      echo "metadata file not found: $metadata_file (run image build with OUTPUT_DIR set)" >&2
      exit 1
   fi
   digest=$(jq -r '."containerimage.digest"' "$metadata_file")
   if [[ -z "$digest" || "$digest" == "null" ]]; then
      echo "could not extract containerimage.digest from $metadata_file" >&2
      exit 1
   fi
   img="${IMAGE_REPO}/${component}@${digest}"
   echo "signing ${img}"
   # shellcheck disable=SC2086
   cosign sign "$img" --yes ${COSIGN_ARGS:-}
done
